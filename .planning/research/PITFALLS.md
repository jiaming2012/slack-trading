# Domain Pitfalls: Observability + Metabase Analytics for Trading Platform

**Domain:** Full-stack observability (OTel + Grafana) + business analytics (Metabase)
**Researched:** 2026-03-29
**Overall confidence:** MEDIUM-HIGH

---

## Section A: Retained v1.0 Pitfalls (OTel / Grafana)

*(These have already been addressed in v1.0/v1.1 but are retained as reference.)*

### Pitfall 1: OTel Providers Never Initialized (RESOLVED in v1.0)
**Status:** Fixed in Phase 2 of v1.0.

### Pitfall 2-13: OTel ecosystem pitfalls
See original PITFALLS for details. All resolved or mitigated in shipped v1.0/v1.1.

---

## Section B: v2.0 Metabase Pitfalls

### B1: Metabase JVM Eating the Droplet Alive (CRITICAL)

**What goes wrong:** Metabase is a JVM application. On first boot it runs schema sync + fingerprinting against every connected database. Before that completes, the JVM heap can spike past 1.5 GB. On a 4 GB droplet already running Go server, PostgreSQL, EventStoreDB, and otel-lgtm, there is no headroom for an unconstrained JVM.

**Why it happens:** By default, the JVM takes 25–50% of host RAM as its default `-Xmx`. On a 4 GB host, Java will happily claim 1–2 GB without any Docker memory limit set. Combine this with the fingerprinting scan on startup (sampling 10,000 rows per column from `order_records`) and you get OOM kills.

**Consequences:** The OS kills Metabase mid-startup or, worse, kills the Go trading server. Docker Compose restarts loop, causing Postgres connection storms.

**Prevention:**
- Set `-Xmx1g` via `JAVA_OPTS=-Xmx1g` in Docker Compose — leaves ~1 GB each for Go server, Postgres, EventStoreDB, and otel-lgtm.
- Set a hard Docker memory limit: `mem_limit: 1.5g` — prevents runaway heap from competing with trading server.
- Monitor with `docker stats` before and after adding Metabase.

**Detection:** Watch `docker stats` during Metabase startup. If container memory climbs past 1.2 GB, something is misconfigured. Sawtooth pattern (rapid grow, GC, rapid grow) signals insufficient heap headroom.

**Phase:** Address in Phase 1 (Deploy Metabase). Set resource limits before any other work.

---

### B2: Metabase Application DB Mixed with Trading DB (CRITICAL)

**What goes wrong:** Pointing `MB_DB_*` at the same PostgreSQL instance and database (`playground`) that the Go server writes to, without isolating the Metabase application schema. Metabase creates its own tables (`metabase_database`, `report_card`, `dashboards`, etc.) and runs frequent internal queries that compete with live trading queries.

**Why it happens:** Simplest Docker Compose setup uses one Postgres container. Operators reuse the existing `playground` DB.

**Consequences:** Metabase internal migrations and health checks add connection overhead to the `playground` DB. On a 4 GB droplet with `max_connections=100` (Postgres default), Metabase's 15-connection pool plus application pool can saturate connections. Longer-term: Metabase schema migrations can block DDL on shared tables.

**Prevention:**
- Create a dedicated `metabase` database in the same Postgres instance: `CREATE DATABASE metabase;`
- Set `MB_DB_DBNAME=metabase` — Metabase owns this DB entirely.
- The Go server's `playground` DB remains uncontested for application workloads.
- Both DBs are still on the same Postgres process; adjust `max_connections` to at least 150 if running both.

**Detection:** Check `SELECT count(*) FROM pg_stat_activity WHERE datname = 'playground';` during Metabase startup. If count is unexpectedly high, Metabase is using the wrong DB.

**Phase:** Address in Phase 1 (Deploy Metabase) before anything else.

---

### B3: Metabase Schema Sync Locking Postgres During Market Hours (CRITICAL)

**What goes wrong:** Metabase runs an automatic sync + fingerprint scan on every connected database at startup and then periodically (hourly by default). The fingerprint scan runs `SELECT * FROM <table> LIMIT 10000` on every column in every table. On the `order_records` table (potentially 100K+ rows across 37+ playgrounds with JSON `Attributes` column), this is a full sequential scan of a JSONB column.

**Why it happens:** Metabase's sync is designed for read-model analytics databases, not live OLTP trading tables. JSONB columns are especially expensive to fingerprint — PostgreSQL must deserialize each JSON blob to sample values.

**Consequences:** Full sequential scans of `order_records` and `trade_records` during sync compete with the Go server's live order queries. Latency spikes on the Twirp RPC server during market hours. On a 4 GB droplet, the combination of scan I/O + Metabase heap expansion has caused Metabase itself to OOM (documented in metabase/metabase#12060).

**Prevention:**
- In Metabase Admin → Databases → your trading DB: disable "Periodically refingerprint tables" (off by default, verify it stays off).
- Hide tables Metabase should not touch: set `order_closes`, `trade_closed_by`, `order_reconciles` (join tables) to Hidden visibility. Metabase will skip them.
- Add a `CREATE INDEX CONCURRENTLY idx_order_records_playground_ts ON order_records(playground_id, timestamp);` before exposing the table to Metabase. This prevents Metabase's sampling from triggering full scans.
- If sync is still slow, disable JSON unfolding for the `attributes` JSONB column in Metabase field settings.

**Detection:** Run `SELECT query, state, wait_event_type FROM pg_stat_activity WHERE state = 'active';` when Metabase is syncing. If you see `SELECT * FROM order_records LIMIT 10000` type queries, fingerprinting is running. Check timing with `EXPLAIN ANALYZE`.

**Phase:** Address in Phase 2 (Connect Trading DB). The index must be created before connecting the DB to Metabase.

---

### B4: Spread P&L Grouping — Multi-Leg Orders Are Not Atomic in the DB (HIGH)

**What goes wrong:** The `order_records` table uses `class = 'multileg_option'` to tag spread orders. However, in the current schema, legs of a multi-leg spread are stored as individual `OrderRecord` rows linked via `order_closes` M2M. Metabase's query builder has no concept of "group these rows into one spread trade." A naive SQL P&L aggregation (`SUM(quantity * price)`) will double-count legs or produce nonsensical P&L numbers.

**Why it happens:** The data model is correct for order lifecycle tracking (each leg needs individual status, fill price, timestamps) but inconvenient for analytics aggregation. Metabase's GUI question builder operates row-by-row; it cannot join across the M2M table automatically.

**Concrete example — covered call entry:**
- Leg 1: `buy_to_open` 100 shares of AAPL (equity)
- Leg 2: `sell_to_open` 1 AAPL call option

If you `SUM(trade_records.price * trade_records.quantity)` without spread grouping, the stock leg's cost (-$18,500) and the option premium (+$320) appear as separate P&L contributions that are meaningless individually.

**Consequences:** P&L dashboards show incorrect values. Win rate calculations count individual legs as separate trades. Strategy comparison is meaningless.

**Prevention:**
- Create a SQL view `spread_trades` that groups legs by `close_order_id` (or a new `spread_id` tag in the `Tag` or `Attributes` column) and aggregates total debit/credit, entry timestamp, and exit timestamp.
- Use the `Tag` field (already present in `OrderRecord`) to stamp a spread group identifier at order creation time: `tag = "spread_<uuid>"`. All legs of the spread share the same tag.
- In Metabase, build all P&L questions against `spread_trades` view, not raw `order_records`.
- For the covered call strategy specifically: the equity leg and option leg must be treated as a combined cost basis, not separate P&L events.

**Detection:** Build a test P&L dashboard with known backtest data. If `total_pnl` != `sum(actual_trade_outcomes)`, the grouping is wrong.

**Phase:** Address in Phase 3 (P&L Analytics). Must design the spread view before building any P&L dashboard. This is the highest-risk domain-specific pitfall.

---

### B5: Partial Fill Creates Phantom P&L (HIGH)

**What goes wrong:** When an option order is partially filled (`quantity < requested_quantity`), the `TradeRecord` stores only the filled quantity. However, the open position cache stores the net position. If a P&L query computes `SUM(trade_records.price * trade_records.quantity)` without accounting for remaining open position, it shows realized P&L on a position that is not fully closed.

**Why it happens:** `TradeRecord` rows are fills, not complete order outcomes. The distinction between "filled 5 of 10 contracts" and "fully filled 10 contracts" is implicit in the order status (`partially_filled` vs `filled`), not in the trade record itself.

**Consequences:** Metabase dashboards show positive realized P&L on positions that are still open. This is especially dangerous for spread analysis where one leg is filled and the other is not.

**Prevention:**
- Filter P&L views to only include `order_records` with `status = 'filled'` (fully filled) when computing closed-trade P&L.
- Track open P&L separately by querying the current position from the positions cache (this requires either a snapshotted positions table or a live query).
- Add a `realized_pnl` column to a materialized analytics table updated on each fill.

**Phase:** Address in Phase 3 (P&L Analytics) when designing the spread view.

---

### B6: Early Option Assignment Breaks P&L Attribution (MODERATE)

**What goes wrong:** If a short call option in a covered call spread is assigned early (before expiration), the Go broker records an assignment as a special order type. The P&L accounting changes: the option premium is realized, but the stock leg now has a different cost basis. If Metabase queries treat the original stock purchase and the assignment as independent events, the P&L for the spread is wrong.

**Why it happens:** Early assignment converts a multi-leg strategy into two separate single-leg events mid-lifecycle. Most P&L models assume legs close in the originally planned way.

**Consequences:** Strategy-level P&L shows the stock sale at assignment price without netting against the original call premium. Win rate for covered calls appears inflated or deflated depending on assignment price.

**Prevention:**
- The `Closes` M2M relationship on `OrderRecord` must be followed when computing spread P&L: the closing order's trades net against the opening order's trades.
- For assignment events: ensure the Go server tags the assignment order with the same `spread_id` (via `Tag` or `Attributes`) as the original covered call entry.
- Write a test case: one covered call with early assignment → verify spread P&L view shows correct combined outcome.

**Phase:** Address in Phase 4 (Spread-Aware Analytics) as a specific edge case test.

---

### B7: Backtest Playground Volume — Postgres Row Count and Query Lag (MODERATE)

**What goes wrong:** Each backtest run creates a Playground with its own set of `order_records`, `trade_records`, and `equity_plot_records`. With 37 playgrounds already in the DB, an active backtest optimization run (e.g., 100 parameter combinations × 252 trading days × N signals/day) can generate millions of rows. Metabase queries over unindexed `playground_id` columns on large tables become progressively slower.

**Why it happens:** GORM auto-migrate adds primary keys but not composite indexes for analytics query patterns. Metabase scans `order_records` filtered by `playground_id` for per-strategy P&L, and without an index on `(playground_id, timestamp)`, this is a sequential scan.

**Consequences:** P&L dashboards take 30+ seconds to load. Postgres I/O spikes during backtest runs when Metabase simultaneously queries the same tables.

**Prevention:**
- Add composite indexes before backtest volume grows:
  ```sql
  CREATE INDEX CONCURRENTLY idx_or_pg_ts ON order_records(playground_id, timestamp);
  CREATE INDEX CONCURRENTLY idx_tr_pg_ts ON trade_records(order_id, timestamp);
  CREATE INDEX CONCURRENTLY idx_epr_pg_ts ON equity_plot_records(playground_id, timestamp);
  ```
- Consider `BRIN` indexes on timestamp columns for append-only historical data (much smaller, nearly free).
- For strategy optimization runs: write results to a separate `backtest_summary` table (one row per run) rather than querying raw `order_records` in Metabase.

**Phase:** Address in Phase 2 (Connect Trading DB) — indexes before Metabase queries the tables.

---

### B8: H2 Default App Database Loses All Dashboards on Container Restart (MODERATE)

**What goes wrong:** Metabase's default application database is H2 (embedded file-based). If the Docker container is not configured with a persistent volume, or if the container is replaced (as happens during `docker-compose up --force-recreate`), all saved questions, dashboards, and data source configurations are lost.

**Why it happens:** The H2 file lives inside the container at `/metabase-data/` by default. Operators configure Metabase with their trading dashboards, restart for an update, and everything is gone.

**Consequences:** All Metabase configuration lost. Hours of dashboard-building work wiped on any container recreation.

**Prevention:**
- Use PostgreSQL as the Metabase app database from day one (`MB_DB_TYPE=postgres`). Never run H2 in production.
- If H2 is used accidentally during dev, migrate to Postgres before building real dashboards.
- Mount a named Docker volume for `/metabase-data/` even when using Postgres (for logs and any transient files).

**Phase:** Address in Phase 1 (Deploy Metabase). Non-negotiable before any configuration work.

---

### B9: Metabase Connection Pool Exhausting Postgres max_connections (MODERATE)

**What goes wrong:** Metabase opens up to 15 connections per connected database by default. With two databases connected (app DB + trading DB), that is 30 connections just from Metabase. The Go server's GORM pool adds another 10–20. EventStoreDB's Postgres journal adds a few more. On a default Postgres install (`max_connections = 100`), this leaves limited headroom for active trading queries.

**Why it happens:** PostgreSQL's default `max_connections` is conservative for embedded/small installs. Multiple connection-hungry services easily saturate it.

**Consequences:** Go server receives `FATAL: sorry, too many clients already` under load. Live trading orders may fail to persist. Metabase shows "connection refused" errors.

**Prevention:**
- Set `max_connections = 200` in Postgres config (`/etc/postgresql/postgresql.conf` or `POSTGRES_EXTRA_FLAGS` in Docker).
- Set `MB_APPLICATION_DB_MAX_CONNECTION_POOL_SIZE=5` to reduce Metabase's app DB pool.
- Set `MB_DB_CONNECTION_POOL_SIZE=8` for the analytics (trading) DB connection in Metabase.
- Monitor with: `SELECT count(*) FROM pg_stat_activity GROUP BY datname;`

**Phase:** Address in Phase 1 (Deploy Metabase). Validate connection counts before adding the trading DB as a data source.

---

### B10: Metabase Caching on Free/OSS Tier Requires Manual Refresh (MINOR)

**What goes wrong:** Metabase OSS (the free tier) does not support automatic cache refresh. The cache is populated lazily: the first user to visit a question after cache expiry waits for the full query. For complex P&L aggregations over large tables, this wait can be 10–30 seconds.

**Why it happens:** Pro/Enterprise feature — automatic background cache refresh is paid. OSS only caches on first visit.

**Consequences:** Trading dashboards feel slow for the first load each session. Not a data correctness issue but materially degrades UX.

**Prevention:**
- Set question caching TTL to 1 hour for historical P&L questions (they don't change).
- Keep dashboards with real-time data (order flow) un-cached (no TTL) so they always show current state.
- For strategy comparison (backtest results), cache indefinitely with manual invalidation when new backtests complete.
- Consider Metabase Pro ($500/month) only if the team grows beyond 2-3 users regularly using dashboards.

**Phase:** Address in Phase 5 (Dashboard Polish) as a tuning step, not a blocker.

---

### B11: Options Symbol Format Incompatible with Standard SQL Analytics (MINOR)

**What goes wrong:** Options symbols in the `order_records.symbol` column use OCC format: `AAPL230120C00150000` (underlying + expiry + call/put + strike). Metabase's query builder "starts with" / "contains" filters work fine, but grouping by strike or expiry requires parsing this string — not possible in the GUI question builder without a SQL view.

**Why it happens:** The trading system's symbol format is broker-native (Tradier/OCC). Analytics tools expect normalized columns (underlying, expiry, strike, option_type as separate fields).

**Consequences:** Cannot build "P&L by strike price" or "win rate by expiry date" charts in Metabase's GUI without custom SQL. Users fall back to raw SQL for every options-specific question.

**Prevention:**
- Create a SQL view `option_legs` that parses the OCC symbol into columns:
  ```sql
  CREATE VIEW option_legs AS
  SELECT
    id,
    playground_id,
    symbol AS raw_symbol,
    substring(symbol, 1, length(symbol)-15) AS underlying,
    to_date(substring(symbol, length(symbol)-14, 6), 'YYMMDD') AS expiry_date,
    substring(symbol, length(symbol)-8, 1) AS option_type,  -- 'C' or 'P'
    (substring(symbol, length(symbol)-7)::numeric / 1000) AS strike_price,
    ...
  FROM order_records
  WHERE class IN ('option', 'multileg_option');
  ```
- Build all options analytics questions against `option_legs`, not `order_records`.

**Phase:** Address in Phase 2 (Connect Trading DB) when creating the analytics view layer.

---

## Phase-Specific Warnings (v2.0)

| Phase Topic | Pitfall | Priority | Mitigation |
|-------------|---------|----------|------------|
| Deploy Metabase | B1: JVM OOM on 4 GB droplet | CRITICAL | Set `-Xmx1g`, `mem_limit: 1.5g` in compose |
| Deploy Metabase | B8: H2 app DB lost on restart | CRITICAL | Use Postgres app DB from first boot |
| Deploy Metabase | B2: App DB mixed with trading DB | CRITICAL | Dedicated `metabase` database |
| Deploy Metabase | B9: Connection pool saturation | MODERATE | Set `max_connections=200`, cap Metabase pool |
| Connect Trading DB | B3: Schema sync locks OLTP | CRITICAL | Disable refingerprinting, hide join tables, add index |
| Connect Trading DB | B7: Backtest row volume | MODERATE | Add composite indexes before connecting DB |
| Connect Trading DB | B11: OCC symbol not parseable | MINOR | Create `option_legs` view |
| P&L Analytics | B4: Multi-leg spread double-count | HIGH | Create `spread_trades` view with leg grouping |
| P&L Analytics | B5: Partial fill phantom P&L | HIGH | Filter to `status='filled'` only |
| Spread Analytics | B6: Early assignment breaks attribution | MODERATE | Tag assignment orders with spread_id |
| Dashboard Polish | B10: No auto-cache refresh (OSS) | MINOR | Set TTL by question type |

---

## Sources

- [Metabase production deployment guide](https://www.metabase.com/learn/metabase-basics/administration/administration-and-operation/metabase-in-production) — memory, connection pool recommendations (HIGH confidence)
- [Metabase JVM troubleshooting docs](https://www.metabase.com/docs/latest/troubleshooting-guide/running) — -Xmx configuration, sawtooth GC pattern (HIGH confidence)
- [Metabase sync/scan docs](https://www.metabase.com/docs/latest/databases/sync-scan) — fingerprinting behavior, how to disable (HIGH confidence)
- [Metabase configuring app database](https://www.metabase.com/docs/latest/installation-and-operation/configuring-application-database) — H2 vs Postgres, migration (HIGH confidence)
- [Metabase PostgreSQL connection docs](https://www.metabase.com/docs/latest/databases/connections/postgresql) — connection pool defaults (HIGH confidence)
- [GitHub: base memory increasing after sync #12060](https://github.com/metabase/metabase/issues/12060) — confirmed heap growth during schema sync (MEDIUM confidence)
- [GitHub: long startup due to sync #19604](https://github.com/metabase/metabase/issues/19604) — startup time with multiple databases (MEDIUM confidence)
- [Metabase caching docs](https://www.metabase.com/docs/latest/configuring-metabase/caching) — OSS caching limitations, Pro auto-refresh (HIGH confidence)
- [Assignment risk on limited-risk spreads](https://www.tradestation.com/learn/market-basics/options/understanding-the-risks/assignment-risk-on-limited-risk-options-spreads/) — early assignment edge cases (MEDIUM confidence)
- Codebase analysis: `order_record.go`, `trade_record.go`, `backtester_order_class.go` — confirmed multi-leg model structure (HIGH confidence — direct inspection)

---

*Pitfalls research for: v2.0 Metabase Analytics on existing Go/Postgres/Docker trading platform*
*Researched: 2026-03-29*
