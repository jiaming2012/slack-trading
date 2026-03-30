# Feature Landscape: Metabase Trading Analytics

**Domain:** Business-level trading performance analytics via Metabase on Postgres
**Researched:** 2026-03-29
**Context:** Subsequent milestone — Grafana/OTel handles operational observability (live signal, heartbeat, error rates). Metabase handles business analytics (P&L, strategy comparison, backtest results). They serve different audiences and different questions.

---

## What Metabase Does Well (and What It Doesn't)

Metabase is a BI tool built around SQL questions and dashboards. It excels at:
- Ad-hoc SQL queries on Postgres tables with interactive filters
- Aggregation tables (GROUP BY strategy, symbol, date range)
- Time-series charts from pre-aggregated data
- Dashboard filters with `{{variable}}` and `[[optional clause]]` syntax
- Saved questions reused across dashboards

Metabase does NOT do real-time streaming, complex application logic, or cross-process joins. All analytics here are computed from the Postgres tables that already exist: `playgrounds`, `order_records`, `trade_records`, `equity_plot_records`, `live_accounts`.

---

## Existing Postgres Schema (Analytics-Relevant Columns)

### `playgrounds` table (mapped from `Meta` struct)
| Column | Type | Analytics Use |
|--------|------|--------------|
| `id` (UUID) | uuid | Join key |
| `client_id` | text | Strategy identifier / filter |
| `environment` | text | Filter: `simulator` vs `live` |
| `live_account_type` | text | Filter: `simulator`, `paper`, `margin`, `mock` |
| `start_at` | timestamptz | Backtest date range |
| `end_at` | timestamptz | Backtest date range |
| `symbols` | text[] | Per-symbol filter |
| `tags` | text[] | Grouping / labeling runs |
| `starting_balance` | numeric | ROI denominator |
| `source_broker` | text | Broker filter |
| `deleted_at` | timestamptz | Soft delete |

### `order_records` table
| Column | Type | Analytics Use |
|--------|------|--------------|
| `id` | uint | PK |
| `playground_id` | uuid | Join to playgrounds |
| `class` | text | `equity` vs `option` split |
| `symbol` | text | Per-symbol analytics |
| `side` | text | `buy/sell/buy_to_open/sell_to_open/etc` — open vs close detection |
| `quantity` | numeric | Position sizing |
| `order_type` | text | Market vs limit |
| `requested_price` | numeric | Slippage computation |
| `price` | numeric | Actual fill price (from trades) |
| `status` | text | Filter to `filled` |
| `timestamp` | timestamptz | Trade timing |
| `tag` | text | Strategy tag / signal label |
| `attributes` | jsonb | Arbitrary KV: includes `ev` (expected value) |
| `is_adjustment` | bool | Filter out adjustment orders |
| `is_system_order` | bool | Filter out system orders |
| `previous_balance` | numeric | Balance before order (for incremental equity) |
| `close_order_id` | uint | Links closing order to opening order |

### `trade_records` table
| Column | Type | Analytics Use |
|--------|------|--------------|
| `id` | uint | PK |
| `order_id` | uint | FK to order_records |
| `timestamp` | timestamptz | Exact fill time |
| `quantity` | numeric | Fill size |
| `price` | numeric | Fill price |
| `parent_trade_id` | uint | Partial fill parent |

### `equity_plot_records` table
| Column | Type | Analytics Use |
|--------|------|--------------|
| `playground_session_id` | uuid | Join to playgrounds |
| `timestamp` | timestamptz | Time axis |
| `equity` | numeric | Account value over time |

### Join tables
- `order_closes` — M2M: which opening order was closed by which closing order
- `trade_closed_by` — M2M: which trade filled which closing order

---

## Table Stakes

Features a trading analytics dashboard must have. Missing = the dashboard is not worth opening.

| Feature | Why Expected | Complexity | Postgres Dependency |
|---------|--------------|------------|---------------------|
| Total realized P&L per playground | Core performance number — is this strategy profitable? | Low | `order_records.pl` or derived from trade_records via order_closes |
| Win rate (winners / total closed trades) | Fundamental metric. <40% signals review needed. | Low | COUNT grouped by playground_id, class |
| Profit factor (gross_profit / abs(gross_loss)) | Industry-standard measure of strategy quality. >1.5 = healthy. | Low | SUM(CASE WHEN pl > 0) / ABS(SUM(CASE WHEN pl < 0)) |
| Average winning trade / average losing trade | Shows reward:risk ratio — context for win rate | Low | AVG filtered by pl > 0, pl < 0 |
| Equity curve chart | Visual summary of whether the account is growing or degrading over time | Low | `equity_plot_records` — timestamp + equity, grouped by playground |
| Trade count by symbol | Which symbol drove most activity? | Low | GROUP BY symbol, class |
| Slippage summary (open + close) | Is the simulation fill assumption realistic? | Medium | requested_price vs trade fill price per order |
| Playground filter (dropdown) | Browse one backtest at a time | Low | Metabase `{{playground_id}}` variable |
| Date range filter | Focus on specific time windows | Low | Metabase `[[AND timestamp >= {{start}}]]` optional clauses |
| Open vs closed position table | What is still open at end of a run? | Medium | filter_open_orders logic: orders where closed volume < quantity |

## Differentiators

Features that go beyond basic metrics. These turn a report into an analytical tool.

| Feature | Value Proposition | Complexity | Postgres Dependency |
|---------|-------------------|------------|---------------------|
| Backtest comparison table | Compare 5+ runs side-by-side: P&L, win rate, profit factor, sharpe, drawdown. The strategy optimization workflow. | Medium | Aggregate query grouped by playground_id with JOIN to playgrounds for metadata (tags, symbols, date range) |
| Spread-aware P&L (multi-leg grouping) | Options covered calls / credit spreads are 2+ legs. Per-leg P&L is misleading. Group by `close_order_id` chain to compute round-trip P&L as a single trade unit. | High | Requires joining order_closes + trade_closed_by to pair open/close legs. Existing `playground_metrics.py` has the logic — needs SQL translation. |
| Per-symbol P&L breakdown | "AAPL covered calls earned $X; TSLA mean reversion lost $Y" — critical for multi-symbol strategies | Low | GROUP BY symbol JOIN playground |
| Trade duration histogram | Are trades being held too long? Are quick exits dragging down P&L? | Medium | EXTRACT(EPOCH FROM close_time - open_time) grouped into buckets |
| Expected value vs realized P&L | Was the strategy's `ev` attribute (stored in `attributes` JSONB) predictive? EV accuracy = alpha validation. | Medium | CAST(attributes->>'ev' AS numeric) correlated with actual pl |
| Strategy type comparison (tags) | Compare `covered_call` vs `mean_reversion` tags across multiple playground runs | Low | GROUP BY tags (unnest array), JOIN to playground metadata |
| Max drawdown per run | Downside risk beyond what win rate shows | Medium | Window function: MAX equity - MIN subsequent equity per playground |
| Sharpe ratio per run | Risk-adjusted return. Requires time-series of daily returns. | High | Derived from equity_plot_records: daily return = (equity[t] - equity[t-1]) / equity[t-1], then AVG/STDDEV |
| Backtest persistence status | Which simulator playgrounds have been saved vs are ephemeral? | Low | COUNT of playgrounds with environment='simulator' AND end_at IS NOT NULL |
| Rejection rate analysis | How often are orders rejected and why? Reveals config issues in live sim. | Low | COUNT where status='rejected' GROUP BY reject_reason |

## Anti-Features

Features to explicitly NOT build in Metabase for this milestone.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Real-time live order monitoring | Metabase queries on-demand — not a streaming dashboard. Grafana already does this with OTel. Duplicating it in Metabase is wasted effort. | Keep in Grafana. Grafana owns live/operational views. |
| Greeks aggregation (Delta, Theta, Vega) | Greeks require current market prices to be meaningful. Metabase queries Postgres, not live market data. Stored Greeks are stale within minutes. | If Greeks are needed, add a Grafana panel that reads a live metric pushed from the trading engine. |
| Intraday charting (candlesticks) | Metabase is not a charting platform. OHLCV rendering needs a specialized tool. | Grafana with a time series panel from Prometheus metrics already handles tick-level charts. |
| Alerts / notifications | Metabase OSS does not have production-grade alerting. Grafana unified alerting already handles this. | Grafana alert rules. |
| Portfolio-level live P&L (mark-to-market) | Requires real-time price lookups. Metabase can't call Tradier/Polygon from a dashboard. | Show settled/realized P&L only in Metabase. |
| User authentication / multi-user access control | Single operator — adds complexity with no benefit now. | Leave Metabase on default admin auth, restricted to the droplet network. |
| Custom Metabase plugins / extensions | Fragile, OSS-only limitation, unnecessary for these use cases. | SQL custom questions cover all needed analytics. |

---

## Feature Dependencies

```
Metabase deployed (Docker Compose) → All dashboard features
Postgres accessible from Metabase container → All SQL questions

order_records (filled, with close_order_id populated) → P&L metrics, slippage, win rate
equity_plot_records (populated by simulator) → Equity curve, drawdown, Sharpe
playgrounds.tags (set on CreatePlayground) → Strategy comparison by tag

order_closes M2M table → Spread-aware P&L (multi-leg grouping)
trade_closed_by M2M table → Round-trip trade matching

Simulator persistence (backtest results saved to Postgres) → Backtest comparison
  └── If simulator playgrounds currently NOT persisted, persistence must ship first
```

**Critical dependency to verify:** Confirm that `environment='simulator'` playgrounds currently persist `equity_plot_records` and `order_records` to Postgres (not just in-memory). The `playground_metrics.py` tool fetches data via Twirp RPC (not direct SQL), which suggests the data may live only in memory during a run. If so, **backtest persistence is a prerequisite feature** before analytics is possible.

---

## MVP Recommendation

Prioritize (in order):

1. **Metabase deployed** — Docker Compose, Postgres connection string, basic auth. Nothing else matters until this works.
2. **Core performance dashboard** — Single playground view: total P&L, win rate, profit factor, trade count, equity curve. These are SQL aggregations on data that already exists.
3. **Slippage analysis** — `requested_price` vs fill price per order side. Critical for validating simulation assumptions. Existing `playground_metrics.py` has the formulas.
4. **Backtest comparison table** — Multi-row table with one row per playground (filtered by tag). Drives strategy optimization workflow directly.
5. **Spread-aware P&L** — Multi-leg option trade grouping. Medium complexity due to M2M joins, but this is the biggest gap in the existing Python tooling (the `sell_to_close` / `buy_to_cover` path is commented out in `playground_metrics.py`).

Defer:
- **Sharpe ratio**: Requires daily-bucketed equity_plot_records and window functions. High complexity, low urgency for MVP.
- **Trade duration histogram**: Nice-to-have, medium complexity. Deliver after core metrics work.
- **Expected value vs realized correlation**: Interesting but requires JSONB casting and statistical interpretation. Post-MVP.

---

## Complexity Reference

| Level | What It Means in Metabase |
|-------|--------------------------|
| Low | Single-table SQL GROUP BY or filter. No joins. Metabase query builder can handle it. |
| Medium | 2-3 table JOIN, window functions, or conditional aggregation. Requires native SQL question. |
| High | Multi-step CTEs, recursive joins across M2M tables (order_closes + trade_closed_by), or requires data not currently in Postgres. May need a Postgres VIEW created first. |

---

## Sources

- [Metabase SQL in Metabase — Official Learn](https://www.metabase.com/learn/metabase-basics/querying-and-dashboards/sql-in-metabase/)
- [Metabase Dashboard Best Practices](https://www.metabase.com/learn/metabase-basics/querying-and-dashboards/dashboards/bi-dashboard-best-practices)
- [Metabase Optional Variables (filter syntax)](https://www.metabase.com/docs/latest/questions/native-editor/optional-variables)
- [Top 7 Backtesting Metrics — LuxAlgo](https://www.luxalgo.com/blog/top-7-metrics-for-backtesting-results/)
- [TradesViz — Multi-leg Options Journaling](https://www.tradesviz.com/how-to-journal-vertical-spreads/)
- [Profit Factor Definition and Benchmarks](https://www.backtestbase.com/education/win-rate-vs-profit-factor)
- Existing codebase: `src/clients/python/tools/playground_metrics.py` — authoritative source for metric formulas already in use
- Existing codebase: `src/go/backtester-api/models/order_record.go`, `trade_record.go`, `equity_plot_record.go`, `playground_meta.go` — authoritative Postgres schema

*Feature landscape for: Metabase trading analytics (v2.0 milestone)*
*Researched: 2026-03-29*
