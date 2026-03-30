-- analytics-schema.sql
-- Idempotent migration: composite indexes + analytics views for Metabase dashboards
-- Run against playground database as grodt user:
--   psql -h <HOST> -U grodt -d playground -f infra/analytics-schema.sql
--
-- NOTE: CREATE INDEX CONCURRENTLY cannot run inside a transaction block.
-- Run this file with: psql --single-transaction=off -f infra/analytics-schema.sql
-- Or execute each CREATE INDEX CONCURRENTLY statement individually.
--
-- Safe to run multiple times -- all operations use IF NOT EXISTS / OR REPLACE.

-- =============================================================
-- 1. Composite indexes for analytics query patterns
-- =============================================================

-- Analytics: filter orders by playground + time range (dashboard date pickers)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_order_records_playground_timestamp
  ON order_records (playground_id, timestamp);

-- Analytics: filter orders by playground + status (e.g., only filled orders)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_order_records_playground_status
  ON order_records (playground_id, status);

-- Analytics: trade lookups by order + time
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_records_order_timestamp
  ON trade_records (order_id, timestamp);

-- Analytics: equity curve by playground + time
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_equity_plot_playground_timestamp
  ON equity_plot_records (playground_session_id, timestamp);

-- Join table indexes for P&L view performance
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_order_closes_close_id
  ON order_closes (close_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_closed_by_trade_record_id
  ON trade_closed_by (trade_record_id);

-- =============================================================
-- 2. Analytics views
-- =============================================================

-- ---------------------------------------------------------
-- v_trade_fills: Flattened order+trade join, one row per fill
-- ---------------------------------------------------------
CREATE OR REPLACE VIEW v_trade_fills AS
SELECT
    p.id                     AS playground_id,
    p.client_id,
    p.environment,
    p.tags,
    o.id                     AS order_id,
    o.symbol,
    o.class,
    o.side,
    o.tag                    AS order_tag,
    o.status,
    o.requested_price,
    o.is_system_order,
    o.is_adjustment,
    o.timestamp              AS order_time,
    t.id                     AS trade_id,
    t.timestamp              AS fill_time,
    t.quantity               AS fill_qty,
    t.price                  AS fill_price
FROM order_records o
JOIN trade_records t ON t.order_id = o.id
JOIN playground_sessions p ON p.id = o.playground_id
WHERE o.deleted_at IS NULL
  AND t.deleted_at IS NULL;

-- ---------------------------------------------------------
-- v_order_pnl: Per-order realized P&L replicating CalcRealizedPL()
--
-- Handles 4 order side cases from order_record.go:
--   1. buy/buy_to_open: PL via ClosedBy trades (quantity < 0)
--   2. sell_short/sell_to_open: PL via ClosedBy trades (quantity > 0)
--   3. sell/sell_to_close: PL via Closes (opening orders) + ClosedBy
--   4. buy_to_cover/buy_to_close: PL via Closes + ClosedBy
--
-- GetAvgFillPrice() is a simple average (sum price / count), NOT VWAP.
-- This matches the Go server's CalcRealizedPL() which is authoritative.
-- ---------------------------------------------------------
CREATE OR REPLACE VIEW v_order_pnl AS
WITH order_avg_fill AS (
    -- GetAvgFillPrice: simple average of trade prices (not volume-weighted)
    SELECT
        o.id AS order_id,
        AVG(t.price) AS avg_fill_price
    FROM order_records o
    JOIN trade_records t ON t.order_id = o.id
    WHERE o.deleted_at IS NULL AND t.deleted_at IS NULL
    GROUP BY o.id
),
-- Cases 1 & 2: buy/buy_to_open and sell_short/sell_to_open
-- These use order's own avg fill price vs ClosedBy trades
direct_pnl AS (
    SELECT
        o.id AS order_id,
        CASE
            WHEN o.side IN ('buy', 'buy_to_open') THEN
                SUM(
                    CASE WHEN tcb_t.quantity < 0
                    THEN (tcb_t.price - oaf.avg_fill_price) * ABS(tcb_t.quantity)
                    ELSE 0 END
                )
            WHEN o.side IN ('sell_short', 'sell_to_open') THEN
                SUM(
                    CASE WHEN tcb_t.quantity > 0
                    THEN (oaf.avg_fill_price - tcb_t.price) * tcb_t.quantity
                    ELSE 0 END
                )
            ELSE 0
        END AS realized_pl
    FROM order_records o
    JOIN order_avg_fill oaf ON oaf.order_id = o.id
    LEFT JOIN trade_closed_by tcb ON tcb.order_record_id = o.id
    LEFT JOIN trade_records tcb_t ON tcb_t.id = tcb.trade_record_id
    WHERE o.deleted_at IS NULL
      AND o.side IN ('buy', 'buy_to_open', 'sell_short', 'sell_to_open')
      AND o.status = 'filled'
    GROUP BY o.id, o.side
),
-- Cases 3 & 4: sell/sell_to_close and buy_to_cover/buy_to_close
-- Iterate over Closes (opening orders), get their avg fill price,
-- then match ClosedBy trades belonging to the closing order.
-- NOTE: This CTE is best-effort for sell/buy_to_cover sides.
-- The tradeMatchesOrder() logic in Go checks if a ClosedBy trade's ID
-- matches a closing order's trade ID, or if parent_trade_id matches.
close_pnl AS (
    SELECT
        closing_o.id AS order_id,
        CASE
            WHEN closing_o.side IN ('sell', 'sell_to_close') THEN
                SUM((tcb_t.price - open_oaf.avg_fill_price) * ABS(tcb_t.quantity))
            WHEN closing_o.side IN ('buy_to_cover', 'buy_to_close') THEN
                SUM((open_oaf.avg_fill_price - tcb_t.price) * ABS(tcb_t.quantity))
            ELSE 0
        END AS realized_pl
    FROM order_records closing_o
    JOIN order_closes oc ON oc.close_id = closing_o.id
    JOIN order_records open_o ON open_o.id = oc.order_record_id
    JOIN order_avg_fill open_oaf ON open_oaf.order_id = open_o.id
    LEFT JOIN trade_closed_by tcb ON tcb.order_record_id = open_o.id
    LEFT JOIN trade_records tcb_t ON tcb_t.id = tcb.trade_record_id
    -- Match: the ClosedBy trade must belong to the closing order's trades
    LEFT JOIN trade_records closing_t ON closing_t.order_id = closing_o.id
        AND (tcb_t.id = closing_t.id OR tcb_t.parent_trade_id = closing_t.id)
    WHERE closing_o.deleted_at IS NULL
      AND closing_o.side IN ('sell', 'sell_to_close', 'buy_to_cover', 'buy_to_close')
      AND closing_o.status = 'filled'
      AND closing_t.id IS NOT NULL  -- only trades matching the closing order
    GROUP BY closing_o.id, closing_o.side
)
SELECT
    o.id AS order_id,
    o.playground_id,
    o.symbol,
    o.class,
    o.side,
    o.tag,
    o.timestamp AS order_time,
    o.status,
    CASE
        WHEN o.class = 'option' THEN COALESCE(dp.realized_pl, cp.realized_pl, 0) * 100
        ELSE COALESCE(dp.realized_pl, cp.realized_pl, 0)
    END AS realized_pl
FROM order_records o
LEFT JOIN direct_pnl dp ON dp.order_id = o.id
LEFT JOIN close_pnl cp ON cp.order_id = o.id
WHERE o.deleted_at IS NULL
  AND o.status = 'filled';

-- ---------------------------------------------------------
-- v_playground_stats: Aggregated per-playground metrics
-- Only aggregates opening-side orders to avoid double-counting P&L
-- (closing side orders overlap with opening side CalcRealizedPL).
-- Matches playground_metrics.py which only processes buy/buy_to_open
-- and sell_short/sell_to_open sides.
-- ---------------------------------------------------------
CREATE OR REPLACE VIEW v_playground_stats AS
SELECT
    playground_id,
    COUNT(*) AS total_trades,
    SUM(realized_pl) AS total_pnl,
    COUNT(*) FILTER (WHERE realized_pl > 0) AS winners,
    COUNT(*) FILTER (WHERE realized_pl < 0) AS losers,
    COUNT(*) FILTER (WHERE realized_pl = 0) AS breakeven,
    CASE WHEN COUNT(*) > 0
        THEN COUNT(*) FILTER (WHERE realized_pl > 0)::numeric / COUNT(*)
        ELSE 0
    END AS win_rate,
    SUM(realized_pl) FILTER (WHERE realized_pl > 0) AS gross_profit,
    SUM(realized_pl) FILTER (WHERE realized_pl < 0) AS gross_loss,
    CASE WHEN SUM(realized_pl) FILTER (WHERE realized_pl < 0) <> 0
        THEN SUM(realized_pl) FILTER (WHERE realized_pl > 0) / ABS(SUM(realized_pl) FILTER (WHERE realized_pl < 0))
        ELSE NULL
    END AS profit_factor,
    AVG(realized_pl) FILTER (WHERE realized_pl > 0) AS avg_win,
    AVG(realized_pl) FILTER (WHERE realized_pl < 0) AS avg_loss
FROM v_order_pnl
WHERE side IN ('buy', 'buy_to_open', 'sell_short', 'sell_to_open')
GROUP BY playground_id;

-- ---------------------------------------------------------
-- v_open_slippage: Per-fill slippage (requested vs fill price)
-- ---------------------------------------------------------
CREATE OR REPLACE VIEW v_open_slippage AS
SELECT
    o.id AS order_id,
    o.playground_id,
    o.symbol,
    o.class,
    o.side,
    o.requested_price,
    t.price AS fill_price,
    t.quantity AS fill_qty,
    CASE
        WHEN o.side IN ('buy', 'buy_to_open') THEN t.price - o.requested_price
        WHEN o.side IN ('sell_short', 'sell_to_open') THEN o.requested_price - t.price
        ELSE 0
    END AS slippage_points,
    CASE
        WHEN o.side IN ('buy', 'buy_to_open') THEN (t.price - o.requested_price) * ABS(t.quantity)
        WHEN o.side IN ('sell_short', 'sell_to_open') THEN (o.requested_price - t.price) * ABS(t.quantity)
        ELSE 0
    END AS slippage_dollars
FROM order_records o
JOIN trade_records t ON t.order_id = o.id
WHERE o.deleted_at IS NULL
  AND t.deleted_at IS NULL
  AND o.status = 'filled'
  AND o.side IN ('buy', 'buy_to_open', 'sell_short', 'sell_to_open');

-- ---------------------------------------------------------
-- v_close_slippage: Per-fill slippage for close-side orders
-- Mirrors v_open_slippage but for sell/sell_to_close/buy_to_cover/buy_to_close
-- ---------------------------------------------------------
CREATE OR REPLACE VIEW v_close_slippage AS
SELECT
    o.id AS order_id,
    o.playground_id,
    o.symbol,
    o.class,
    o.side,
    o.requested_price,
    t.price AS fill_price,
    t.quantity AS fill_qty,
    CASE
        WHEN o.side IN ('sell', 'sell_to_close') THEN o.requested_price - t.price
        WHEN o.side IN ('buy_to_cover', 'buy_to_close') THEN t.price - o.requested_price
        ELSE 0
    END AS slippage_points,
    CASE
        WHEN o.side IN ('sell', 'sell_to_close') THEN (o.requested_price - t.price) * ABS(t.quantity)
        WHEN o.side IN ('buy_to_cover', 'buy_to_close') THEN (t.price - o.requested_price) * ABS(t.quantity)
        ELSE 0
    END AS slippage_dollars
FROM order_records o
JOIN trade_records t ON t.order_id = o.id
WHERE o.deleted_at IS NULL
  AND t.deleted_at IS NULL
  AND o.status = 'filled'
  AND o.side IN ('sell', 'sell_to_close', 'buy_to_cover', 'buy_to_close');

-- ---------------------------------------------------------
-- v_all_slippage: Combined open + close slippage with type column
-- ---------------------------------------------------------
CREATE OR REPLACE VIEW v_all_slippage AS
SELECT *, 'open' AS slippage_type FROM v_open_slippage
UNION ALL
SELECT *, 'close' AS slippage_type FROM v_close_slippage;

-- =============================================================
-- 4. Backtest runs summary table
-- =============================================================

CREATE TABLE IF NOT EXISTS backtest_runs (
    id               SERIAL PRIMARY KEY,
    playground_id    UUID NOT NULL REFERENCES playground_sessions(id),
    client_id        TEXT NOT NULL,
    strategy_name    TEXT NOT NULL,
    parameters       JSONB NOT NULL DEFAULT '{}',
    starting_balance NUMERIC NOT NULL,
    final_balance    NUMERIC NOT NULL,
    total_pnl        NUMERIC NOT NULL,
    win_rate         NUMERIC,
    profit_factor    NUMERIC,
    total_trades     INTEGER NOT NULL DEFAULT 0,
    start_date       DATE,
    end_date         DATE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_backtest_runs_strategy
    ON backtest_runs (strategy_name);
CREATE INDEX IF NOT EXISTS idx_backtest_runs_playground
    ON backtest_runs (playground_id);
CREATE INDEX IF NOT EXISTS idx_backtest_runs_created
    ON backtest_runs (created_at DESC);

-- =============================================================
-- 4b. Spread analytics: GIN index, views
-- =============================================================

-- GIN index for JSONB attribute key extraction (spread_group_key, group_id lookups)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_order_records_attributes_gin
  ON order_records USING gin (attributes jsonb_path_ops);

-- v_spread_pnl: Per-spread-group realized P&L
-- Recognizes both server-injected spread_group_key and existing group_id from credit_spread.py
-- Only opening-side orders to avoid double-counting (same filter as v_playground_stats)
CREATE OR REPLACE VIEW v_spread_pnl AS
SELECT
    COALESCE(
        o.attributes->>'spread_group_key',
        o.attributes->>'group_id'
    ) AS spread_key,
    o.playground_id,
    COUNT(*) AS leg_count,
    SUM(pnl.realized_pl) AS net_pnl,
    BOOL_AND(o.status = 'filled') AS all_filled,
    COUNT(*) FILTER (WHERE o.status = 'filled') AS filled_legs,
    MIN(o.timestamp) AS entry_time,
    MAX(o.timestamp) AS last_leg_time,
    ARRAY_AGG(DISTINCT o.symbol) AS symbols,
    ARRAY_AGG(DISTINCT o.attributes->>'leg_role') AS roles
FROM order_records o
JOIN v_order_pnl pnl ON pnl.order_id = o.id
WHERE o.deleted_at IS NULL
  AND (o.attributes->>'spread_group_key' IS NOT NULL
       OR o.attributes->>'group_id' IS NOT NULL)
  AND o.side IN ('buy', 'buy_to_open', 'sell_short', 'sell_to_open')
GROUP BY
    COALESCE(o.attributes->>'spread_group_key', o.attributes->>'group_id'),
    o.playground_id;

-- v_spread_stats: Aggregated spread performance per playground
CREATE OR REPLACE VIEW v_spread_stats AS
SELECT
    playground_id,
    COUNT(*) AS total_spreads,
    SUM(net_pnl) AS total_net_pnl,
    COUNT(*) FILTER (WHERE net_pnl > 0) AS winners,
    COUNT(*) FILTER (WHERE net_pnl < 0) AS losers,
    COUNT(*) FILTER (WHERE net_pnl = 0) AS breakeven,
    CASE WHEN COUNT(*) > 0
        THEN COUNT(*) FILTER (WHERE net_pnl > 0)::numeric / COUNT(*)
        ELSE 0
    END AS win_rate,
    AVG(net_pnl) AS avg_spread_pnl,
    AVG(net_pnl) FILTER (WHERE net_pnl > 0) AS avg_win,
    AVG(net_pnl) FILTER (WHERE net_pnl < 0) AS avg_loss
FROM v_spread_pnl
WHERE all_filled
GROUP BY playground_id;

-- =============================================================
-- 5. Grant SELECT on views and tables to metabase_ro
-- =============================================================
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'metabase_ro') THEN
        GRANT SELECT ON v_trade_fills TO metabase_ro;
        GRANT SELECT ON v_order_pnl TO metabase_ro;
        GRANT SELECT ON v_playground_stats TO metabase_ro;
        GRANT SELECT ON v_open_slippage TO metabase_ro;
        GRANT SELECT ON v_close_slippage TO metabase_ro;
        GRANT SELECT ON v_all_slippage TO metabase_ro;
        GRANT SELECT ON backtest_runs TO metabase_ro;
        GRANT SELECT ON v_spread_pnl TO metabase_ro;
        GRANT SELECT ON v_spread_stats TO metabase_ro;
        RAISE NOTICE 'Granted SELECT on analytics views and backtest_runs to metabase_ro';
    ELSE
        RAISE NOTICE 'Role metabase_ro does not exist -- skipping GRANTs. Run init-metabase.sql first, then re-run this file.';
    END IF;
END
$$;

-- =============================================================
-- Verification queries (run manually to sanity-check)
-- =============================================================
-- Verify indexes exist:
-- SELECT indexname FROM pg_indexes WHERE tablename = 'order_records' AND indexname LIKE 'idx_%';
-- SELECT indexname FROM pg_indexes WHERE tablename = 'trade_records' AND indexname LIKE 'idx_%';
-- SELECT indexname FROM pg_indexes WHERE tablename = 'equity_plot_records' AND indexname LIKE 'idx_%';
-- SELECT indexname FROM pg_indexes WHERE tablename = 'order_closes' AND indexname LIKE 'idx_%';
-- SELECT indexname FROM pg_indexes WHERE tablename = 'trade_closed_by' AND indexname LIKE 'idx_%';
--
-- Verify views exist:
-- SELECT table_name FROM information_schema.views WHERE table_schema = 'public' AND table_name LIKE 'v_%';
--
-- Spot-check v_playground_stats for a known playground:
-- SELECT * FROM v_playground_stats LIMIT 5;
--
-- Verify EXPLAIN shows index usage:
-- EXPLAIN SELECT * FROM order_records WHERE playground_id = '00000000-0000-0000-0000-000000000000' AND timestamp > '2024-01-01';
