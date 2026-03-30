# Architecture Patterns: Metabase Analytics Integration

**Domain:** Business-level trading analytics via Metabase on existing Go+Python platform
**Researched:** 2026-03-29
**Overall confidence:** HIGH

---

## How Metabase Integrates With the Existing Architecture

Metabase is a read-only analytics layer. It connects directly to the existing PostgreSQL database
(port 5432) using a read-only user. It does not write to application tables, does not receive
events from the Go server, and does not replace Grafana. The two observability tools serve
different purposes: Grafana shows real-time operational state (is it alive?), Metabase shows
historical business analytics (how has performance been?).

### Updated Architecture

```
+---------------------+        +---------------------+
| Go Server (grodt)   |        | Python Client       |
| - Twirp RPC :5051   |        | - Trading engine    |
| - REST API :8080    |        | - Strategy logic    |
| - OTel SDK          |        | - OTel SDK          |
+-----+---------------+        +-----------+---------+
      |                                    |
      | writes orders/trades               | Twirp RPC calls
      v                                    |
+-----+------------------------------------+
| PostgreSQL :5432                         |
| - playgrounds (UUID PK)                  |
| - order_records (uint PK)                |
| - trade_records (uint PK)                |
| - equity_plot_records                    |
| + NEW: backtest_runs (metadata table)    |
| + NEW: spread_groups (leg grouping)      |
| + NEW: SQL views for analytics           |
+-----+----+-----------------------------+--
      |    |                             |
      |    | read-only (MB_DB_USER)      | read/write (app user)
      |    |                             |
      v    v                             |
+----------+--------+           +--------+----------+
| Metabase :3001    |           | OTel Collector     |
| - Dashboards      |           +--+---+---+---------+
| - SQL Questions   |              |   |   |
| - Collections     |         Loki Tempo Prometheus
| - Embedded views  |              |   |   |
+-------------------+           +--v---v---v---+
                                | Grafana :3000 |
                                +---------------+
```

### Port Assignments (Updated for 5-Service Stack)

| Service     | Port | Notes |
|-------------|------|-------|
| Grafana     | 3000 | Operational observability — keep |
| Metabase    | 3001 | Business analytics — NEW, mapped from container 3000 |
| OTel Collector | 4317/4318 | Inside otel-lgtm container |
| PostgreSQL  | 5432 | Shared by grodt + Metabase (different DB users) |
| EventStoreDB | 1113/2113 | Unchanged |
| Twirp RPC   | 5051 | Unchanged |
| REST API    | 8080 | Unchanged |

Metabase runs on host port 3001 (maps to container 3000). This avoids collision with Grafana on 3000.

---

## Memory Budget: 4 GB Droplet

Current DO droplet: s-2vcpu-4gb (4 GB RAM total).

### Current Allocations (v1.1)

| Service | Typical Resident | Peak |
|---------|-----------------|------|
| OS + Docker daemon | ~400 MB | ~600 MB |
| EventStoreDB | ~300 MB | ~500 MB |
| PostgreSQL 13 | ~200 MB | ~400 MB |
| otel-lgtm (Grafana+Loki+Tempo+Prometheus+Collector) | ~700 MB | ~1.1 GB |
| grodt (Go server) | ~150 MB | ~300 MB |
| **Total (v1.1)** | ~1.75 GB | ~2.9 GB |

### Adding Metabase

Metabase is a Java (JVM) application. The JVM allocates heap eagerly.

| Metabase heap config | Resident memory | Suitable for |
|---------------------|-----------------|--------------|
| Default (no JAVA_OPTS) | 1.5–2 GB | Fails on 4 GB droplet alongside existing stack |
| `-Xmx512m` | ~700 MB | Single-user analytics, read-only queries — sufficient |
| `-Xmx768m` | ~900 MB | Recommended ceiling for this droplet |

**Decision: Set `JAVA_OPTS=-Xmx768m -Xms256m`**

This caps Metabase heap at 768 MB. Total expected footprint after adding Metabase:

| Component | Resident |
|-----------|---------|
| Existing services | ~1.75 GB |
| Metabase (capped) | ~900 MB |
| Buffer | ~350 MB |
| **Total** | ~3.0 GB |

This leaves ~1 GB headroom — workable, but tight. A 2 GB swap file on the droplet (already
documented in phase 07 deployment steps) provides a safety net if peak usage spikes.

**Add swap if not already present:**
```bash
ssh root@<DROPLET_IP> "fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile && echo '/swapfile none swap sw 0 0' >> /etc/fstab"
```

**Risk:** If otel-lgtm and Metabase both peak simultaneously, the system may use swap. For a
single-operator trading platform this is acceptable — Metabase is used for offline analytics, not
concurrent with peak strategy execution.

---

## Docker Compose Changes

### New Service: metabase

Add to `docker-compose.prod.yaml`:

```yaml
  metabase:
    image: metabase/metabase:v0.52.x   # pin minor version, not latest
    depends_on:
      - postgres
    environment:
      - MB_DB_TYPE=postgres
      - MB_DB_DBNAME=${METABASE_APP_DB}          # separate DB for Metabase state
      - MB_DB_PORT=5432
      - MB_DB_HOST=postgres
      - MB_DB_USER=${METABASE_DB_USER}
      - MB_DB_PASS=${METABASE_DB_PASS}
      - JAVA_OPTS=-Xmx768m -Xms256m
      - MB_SITE_URL=http://${DROPLET_IP}:3001
      - MB_SEND_EMAIL_ON_FIRST_LOGIN=false
    ports:
      - "3001:3000"
    volumes:
      - metabase-data:/metabase-data
    restart: unless-stopped

volumes:
  metabase-data:    # Metabase local state (plugins, temp files)
```

### PostgreSQL Setup Required

Metabase needs two things from PostgreSQL:

1. **Its own application database** (stores Metabase config, questions, dashboards)
2. **Read-only access to the trading data database** (queries order_records, trade_records, etc.)

These are separate concerns using separate DB credentials:

```sql
-- Run once at provision time via init script or manually

-- 1. Metabase application database
CREATE USER metabase_app WITH PASSWORD '<MB_APP_PASS>';
CREATE DATABASE metabaseappdb OWNER metabase_app;

-- 2. Read-only analytics access to trading data
CREATE USER metabase_ro WITH PASSWORD '<MB_RO_PASS>';
GRANT CONNECT ON DATABASE playground TO metabase_ro;
GRANT USAGE ON SCHEMA public TO metabase_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO metabase_ro;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO metabase_ro;
```

In Metabase UI, add a second database connection:
- Host: `postgres` (Docker service name)
- Port: 5432
- DB name: `playground`
- User: `metabase_ro`

This is the "trading data" source used by all dashboards.

---

## Schema Changes for Spread Analytics

### The Problem

The current `order_records` table stores individual legs independently. A covered call has two legs:
- Sell call option (e.g., symbol `AAPL240119C00195000`)
- Buy underlying stock (e.g., symbol `AAPL`)

These are linked only by `tag` (a free-text field) and `close_order_id`. There is no explicit
concept of "spread" or "strategy group" at the database level. This makes it impossible to
compute per-spread P&L in SQL without custom parsing logic.

### New Table: `spread_groups`

```sql
CREATE TABLE spread_groups (
    id              BIGSERIAL PRIMARY KEY,
    playground_id   UUID NOT NULL REFERENCES playgrounds(id) ON DELETE CASCADE,
    group_key       TEXT NOT NULL,          -- composite key: symbol+strategy+expiry (set by strategy)
    strategy_type   TEXT NOT NULL,          -- 'covered_call', 'vertical_spread', 'naked_put', etc.
    underlying      TEXT NOT NULL,          -- e.g., 'AAPL'
    opened_at       TIMESTAMPTZ NOT NULL,
    closed_at       TIMESTAMPTZ,            -- NULL if still open
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(playground_id, group_key)
);
```

```sql
CREATE TABLE spread_group_legs (
    id              BIGSERIAL PRIMARY KEY,
    spread_group_id BIGINT NOT NULL REFERENCES spread_groups(id) ON DELETE CASCADE,
    order_id        BIGINT NOT NULL REFERENCES order_records(id) ON DELETE CASCADE,
    leg_role        TEXT NOT NULL,          -- 'long_stock', 'short_call', 'long_put', etc.
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(spread_group_id, order_id)
);
```

**Why `group_key` not auto-detect:** The strategy code already has this context (it knows it is
placing a covered call at the time of order creation). Pushing grouping logic to the Go server at
order placement time is simpler and more reliable than SQL pattern matching after the fact.

**Migration:** Existing orders have `tag` fields (e.g., `covered_call-AAPL-2024-01-19`). A
one-time backfill script can parse these tags to populate `spread_groups` for historical data.
New orders populate the table at placement time via Go server.

### New Table: `backtest_runs`

The current `playgrounds` table persists simulator playgrounds but is missing analytics-focused
metadata: final P&L, run label, parameter set, duration. Add a companion table:

```sql
CREATE TABLE backtest_runs (
    id              BIGSERIAL PRIMARY KEY,
    playground_id   UUID NOT NULL UNIQUE REFERENCES playgrounds(id) ON DELETE CASCADE,
    label           TEXT,                   -- human-readable name, e.g. 'covered-call-v7-AAPL-2024'
    strategy_name   TEXT NOT NULL,          -- e.g., 'covered_call', 'mean_reversion'
    parameters      JSONB,                  -- strategy parameters as JSON (threshold, DTE, etc.)
    final_balance   NUMERIC,                -- balance at end of simulation
    total_trades    INT,
    win_rate        NUMERIC,
    profit_factor   NUMERIC,
    max_drawdown    NUMERIC,
    completed_at    TIMESTAMPTZ,            -- NULL if backtest is still running
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

This table is written by the Go server when a simulator playground completes (or by the Python
client via a new `SaveBacktestResult` RPC). It does not require Go model GORM changes — it can
be written with a raw SQL upsert after `isBacktestComplete` is set to true.

### SQL Views for Metabase

Create views in PostgreSQL so Metabase dashboards use stable, named views rather than raw table
joins. This insulates dashboards from schema changes.

```sql
-- v_trade_fills: flattened order+trade join, one row per fill
CREATE VIEW v_trade_fills AS
SELECT
    p.id                   AS playground_id,
    p.environment,
    p.tags,
    o.id                   AS order_id,
    o.symbol,
    o.class,
    o.side,
    o.tag                  AS order_tag,
    o.attributes,
    t.id                   AS trade_id,
    t.timestamp            AS fill_time,
    t.quantity             AS fill_qty,
    t.price                AS fill_price,
    t.quantity * t.price   AS fill_notional
FROM order_records o
JOIN trade_records t  ON t.order_id = o.id
JOIN playgrounds p    ON p.id = o.playground_id
WHERE o.deleted_at IS NULL
  AND t.deleted_at IS NULL;
```

```sql
-- v_spread_pnl: per-spread realized P&L via spread_group_legs
CREATE VIEW v_spread_pnl AS
SELECT
    sg.id                   AS spread_group_id,
    sg.playground_id,
    sg.strategy_type,
    sg.underlying,
    sg.opened_at,
    sg.closed_at,
    SUM(
      CASE
        WHEN o.class = 'option' THEN calc_pnl(o.id) * 100
        ELSE calc_pnl(o.id)
      END
    )                       AS realized_pnl,
    COUNT(DISTINCT sgl.order_id) AS leg_count
FROM spread_groups sg
JOIN spread_group_legs sgl ON sgl.spread_group_id = sg.id
JOIN order_records o        ON o.id = sgl.order_id
GROUP BY sg.id, sg.playground_id, sg.strategy_type, sg.underlying, sg.opened_at, sg.closed_at;
```

Note: `calc_pnl` is implemented as a SQL function mirroring the Go `CalcRealizedPL()` logic.
The P&L calculation requires the order's fills and close trades, which are accessible in SQL
via the `order_closes`, `trade_closed_by`, and `trade_records` tables.

```sql
-- v_backtest_summary: backtest runs with playground context
CREATE VIEW v_backtest_summary AS
SELECT
    br.id,
    br.label,
    br.strategy_name,
    br.parameters,
    br.final_balance,
    br.total_trades,
    br.win_rate,
    br.profit_factor,
    br.max_drawdown,
    br.completed_at,
    p.start_at,
    p.end_at,
    p.starting_balance  AS initial_balance,
    p.symbols,
    p.tags
FROM backtest_runs br
JOIN playgrounds p ON p.id = br.playground_id;
```

---

## Data Flow Changes

### New Flow: Spread Group Registration (Go Server)

When Python places a covered call (two legs), the strategy passes a `group_key` attribute in the
order request. The Go server, when persisting the order, also upserts into `spread_groups` and
`spread_group_legs`.

Current `Attributes` field on `OrderRecord` is `map[string]string` stored as JSONB. The strategy
already populates this with `action` tags. Add convention: if `attributes["spread_group_key"]`
is present, the Go server registers the leg.

```
Python strategy
  -> PlaceOrder RPC with attributes: {"spread_group_key": "AAPL-covered-call-2024-01-19", "leg_role": "short_call"}
  -> Go server: order persisted to order_records
  -> Go server: upsert spread_groups (playground_id, group_key, strategy_type, underlying)
  -> Go server: insert spread_group_legs (spread_group_id, order_id, leg_role)
```

### New Flow: Backtest Result Persistence (Go Server)

When a simulator playground finishes (all ticks consumed, `isBacktestComplete = true`), the Go
server computes summary statistics and writes to `backtest_runs`. This replaces the current
situation where backtest results exist only in Python stdout.

The `FinalizeBacktest` logic runs after `NextTick` returns `isBacktestComplete = true`:

```
Python: NextTick() returns {backtest_complete: true}
Go server internal: compute final_balance, total_trades, win_rate, profit_factor, max_drawdown
Go server: INSERT INTO backtest_runs (...) ON CONFLICT (playground_id) DO UPDATE
Metabase: SELECT * FROM v_backtest_summary WHERE completed_at IS NOT NULL
```

---

## Integration Points Summary

| Integration Point | Type | What Changes |
|-------------------|------|-------------|
| `docker-compose.prod.yaml` | NEW service | Add `metabase` service block |
| `.env.prod.template` | NEW vars | `METABASE_APP_DB`, `METABASE_DB_USER`, `METABASE_DB_PASS`, `METABASE_RO_PASS` |
| PostgreSQL | NEW users + DB | `metabase_app` user + `metabaseappdb`, `metabase_ro` read-only user |
| PostgreSQL | NEW tables | `spread_groups`, `spread_group_legs`, `backtest_runs` |
| PostgreSQL | NEW views | `v_trade_fills`, `v_spread_pnl`, `v_backtest_summary` |
| `order_record.go` | UNCHANGED | `Attributes` JSONB field already exists — add convention for `spread_group_key` |
| Go server router | NEW logic | Read `spread_group_key` attribute and write to `spread_groups`/`spread_group_legs` |
| Go server | NEW logic | Write to `backtest_runs` on backtest completion |
| DO Cloud Firewall | NEW rule | Allow TCP 3001 inbound (Metabase UI) |

---

## Component Boundaries: What Metabase Reads vs What It Does Not Touch

| Table / Entity | Metabase Access | Notes |
|----------------|-----------------|-------|
| `order_records` | READ (via `metabase_ro`) | Core data source for trade analytics |
| `trade_records` | READ | Fill prices, quantities, timestamps |
| `playgrounds` | READ | Session metadata, environment, tags |
| `equity_plot_records` | READ | Equity curve over time |
| `backtest_runs` | READ | NEW — backtest summary stats |
| `spread_groups` | READ | NEW — grouping for multi-leg analytics |
| `spread_group_legs` | READ | NEW — leg-to-order mapping |
| `live_accounts` | NOT accessed | Internal broker account management |
| `order_closes`, `trade_closed_by`, `order_reconciles` | READ (via views) | Hidden behind `v_trade_fills`, `v_spread_pnl` |
| OTel telemetry (Loki/Tempo/Prometheus) | NOT accessed | Grafana's domain only |

---

## Build Order (Dependencies)

1. **PostgreSQL setup** — Create `metabase_app`, `metabase_ro` users and `metabaseappdb` database.
   No Go code changes. Unblocks everything else.

2. **New schema migrations** — Add `spread_groups`, `spread_group_legs`, `backtest_runs` tables
   and SQL views. Can be done as a plain SQL migration file or via GORM auto-migrate additions.
   Unblocks: Go server spread registration, Metabase dashboard development.

3. **Docker Compose + Firewall** — Add Metabase service to `docker-compose.prod.yaml`, update
   `.env.prod.template`, open port 3001 in DO Cloud Firewall.
   Depends on: PostgreSQL setup complete (Metabase won't start without its app DB).

4. **Go server: spread group registration** — Read `spread_group_key` from `Attributes` in
   `PlaceOrder` handler, write to new tables.
   Depends on: Schema migrations (tables must exist).

5. **Go server: backtest persistence** — Write to `backtest_runs` when `isBacktestComplete`.
   Depends on: Schema migrations.

6. **Metabase dashboard development** — Build questions and dashboards against the views.
   Depends on: Metabase deployed (step 3) and views accessible (step 2).

7. **Python strategy: emit spread group key** — Pass `spread_group_key` attribute in order
   requests. Aligns with Go server expectation from step 4.
   Depends on: Go server spread registration (step 4).

---

## Scalability Considerations

| Concern | Now (single operator) | Future |
|---------|----------------------|--------|
| Metabase query performance | Negligible. 37 playgrounds, thousands of orders. All fits in PostgreSQL page cache. | Add read replica if query load impacts trading app write throughput. |
| Metabase JVM heap | 768 MB cap is sufficient for 1-5 concurrent users. | Upgrade droplet to 8 GB (s-2vcpu-8gb, ~$2/day more) if needed. |
| `spread_groups` table size | Small. One row per strategy entry. ~100/day at most. | Index on `playground_id, opened_at`. No maintenance needed. |
| `backtest_runs` table size | Very small. One row per backtest. Never more than a few thousand rows. | No concern. |
| View query complexity | `v_spread_pnl` requires joins across 4 tables. Add indexes on `order_closes.close_id`, `trade_closed_by.trade_record_id` if queries are slow. | Materialize `v_spread_pnl` as a scheduled refresh if needed. |

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Metabase Docker integration | HIGH | Official docs verified. `MB_DB_TYPE=postgres`, port 3000 → 3001 mapping, JAVA_OPTS for heap. |
| Memory budget | MEDIUM | Based on community reports of 768m JVM cap being sufficient for single-user. Exact numbers vary by query pattern. Swap provides safety net. |
| Schema design (`spread_groups`) | HIGH | Derived from inspecting actual GORM models. `Attributes` JSONB already exists and is the right injection point. |
| SQL view correctness | MEDIUM | P&L calculation logic mirrors Go's `CalcRealizedPL()`. Needs verification that SQL join traversal matches Go's `ClosedBy` traversal. Validate with known backtest results. |
| Build order dependencies | HIGH | Dependencies are structural (Metabase requires app DB before start). No speculation. |

---

## Sources

- [Metabase running on Docker](https://www.metabase.com/docs/latest/installation-and-operation/running-metabase-on-docker) — environment variables, port 3000 default (HIGH confidence)
- [Metabase minimum resources](https://discourse.metabase.com/t/what-are-metabase-minimum-resources/21470) — 1 CPU + 1 GB baseline, scaling guidance (HIGH confidence)
- [Metabase RAM usage discussion](https://discourse.metabase.com/t/metabase-memory-usage-on-docker/25583) — 2.5 GB available sufficient on 4 GB server (MEDIUM confidence, community reports)
- [Metabase JVM heap configuration](https://discourse.metabase.com/t/apply-metabase-ram-limit-for-jvm-or-docker-container/25471) — JAVA_OPTS -Xmx pattern (HIGH confidence)
- Codebase inspection: `src/go/backtester-api/models/order_record.go` — `Attributes` JSONB field, `CalcRealizedPL()` logic (HIGH confidence)
- Codebase inspection: `src/go/backtester-api/models/playground_meta.go` — `Meta` struct, `Environment`, `Tags`, `InitialBalance` (HIGH confidence)
- Codebase inspection: `docker-compose.prod.yaml` — current 4-service stack, existing port assignments (HIGH confidence)

---

*Architecture research for: Metabase analytics integration on Go+Python trading platform*
*Researched: 2026-03-29*
