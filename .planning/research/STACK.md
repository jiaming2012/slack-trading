# Technology Stack

**Project:** slack-trading (grodt) — Metabase Analytics (v2.0)
**Researched:** 2026-03-29
**Scope:** NEW additions only. Existing validated stack (Go, Python, OTel, Grafana, PostgreSQL, Docker Compose on DigitalOcean) is not re-researched.

---

## What This Milestone Adds

Three new capabilities on top of the shipped v1.1 stack:

1. **Metabase** — Business-level trading analytics dashboard (queries directly against PostgreSQL)
2. **Analytics schema/views in PostgreSQL** — Spread-aware P&L, profit factor, slippage, win rate computed as SQL views
3. **Simulator playground persistence** — Backtest results written to Postgres so Metabase can query historical runs

---

## New Stack Components

### Metabase

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `metabase/metabase` Docker image | v0.59.4 (OSS) | Business analytics UI — dashboards, SQL editor, question builder | Queries Postgres directly; no custom code needed for trading dashboards. v0.59 includes Data Studio (semantic layer) and AI SQL generation in OSS edition. |

**Configuration approach:**
- Metabase's own application state (dashboards, questions, user accounts) stored in a dedicated PostgreSQL database (`metabase` database, same Postgres instance as trading data)
- Trading data is the **data source** connection — same Postgres host, `playground` database, read-only credentials
- Using the same Postgres instance for both is fine on a single-server deployment; the two databases are fully isolated

**Port:** Metabase defaults to 3000, which conflicts with `grafana/otel-lgtm` (also 3000). Resolve by running Metabase on **port 3001** via `MB_JETTY_PORT=3001` and exposing `3001:3000` in Docker Compose. Do NOT change otel-lgtm's port — Grafana provisioning is already tuned to it.

**Memory on 2vCPU/4GB droplet:** Metabase idles at ~600MB and needs ~1-1.5GB under load. The existing stack (Go server, Postgres, EventStoreDB, otel-lgtm) already consumes ~2-2.5GB. Adding Metabase is viable but tight — set `JAVA_OPTS=-Xmx768m` to cap JVM heap and leave headroom for the rest. If the droplet becomes unstable, upgrading to s-2vcpu-8gb is the next step.

**Application database setup:** Must create the `metabase` database before first start:
```sql
CREATE DATABASE metabase WITH ENCODING 'UTF8';
```
This runs once via the existing `infra/init.sql` or a migration task.

**Read-only credentials for data source:** Create a dedicated Postgres user that has SELECT-only on the `playground` database. This is what Metabase uses to query trading data — prevents accidental modification.
```sql
CREATE USER metabase_reader WITH PASSWORD '<secret>';
GRANT CONNECT ON DATABASE playground TO metabase_reader;
GRANT USAGE ON SCHEMA public TO metabase_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO metabase_reader;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO metabase_reader;
```

### PostgreSQL Analytics Schema

No new database engine is needed. Analytics are SQL views on top of existing GORM-managed tables (`order_records`, `trade_records`, `playgrounds`).

**Pattern:** Use a dedicated `analytics` schema in the `playground` database to keep views separate from GORM-managed `public` schema tables. GORM ignores schemas outside `public` unless explicitly configured, so there is zero risk of collision.

```sql
CREATE SCHEMA IF NOT EXISTS analytics;
```

Key views to create:

| View | Purpose | Source Tables |
|------|---------|---------------|
| `analytics.trade_legs` | Flattens order_records with their trade fills | `order_records`, `trade_records` |
| `analytics.spread_groups` | Groups multi-leg option trades by playground + timestamp proximity | `order_records` |
| `analytics.pnl_per_trade` | P&L per completed round-trip (entry + close) using `order_closes` join table | `order_records`, `order_closes`, `trade_records` |
| `analytics.backtest_summary` | Per-playground: profit factor, win rate, total P&L, trade count, duration | `playgrounds`, `analytics.pnl_per_trade` |
| `analytics.equity_curve` | Time-series equity per playground | `equity_plot_records` |

**Spread-aware grouping logic:** The existing `order_closes` many2many join table in the schema already links opening orders to their closing orders. A covered call spread (short call + long stock) can be grouped by `playground_id` + `tag` (which Python already sets via `Attributes` on `OrderRecord`). The `tag` column on `order_records` is the correct grouping key — enforce a convention like `"covered_call_2024-01-15"` at the Python strategy level.

**Migration approach:** Views are idempotent (`CREATE OR REPLACE VIEW`). Add them to `infra/init.sql` (which runs on fresh Postgres start) AND provide a manual migration script for the live droplet. GORM auto-migration does not touch views — no conflict.

### Simulator Playground Persistence

**Current state:** Simulator (backtest) playgrounds are created in-memory only. `CreatePlayground` for `PlaygroundEnvironmentSimulator` does NOT call `SavePlaygroundSession`. Orders and trades are never written to Postgres for simulator runs.

**What's needed:** A `SaveToDB: true` flag path for simulator playgrounds, matching what live and reconcile playgrounds already do. The database models (`Playground`, `OrderRecord`, `TradeRecord`) are already GORM-mapped — the tables exist. It's a behavioral change in the service layer, not a schema change.

**Implementation surface:** `src/go/data/database_service.go` — `CreatePlayground` function, simulator branch (line ~927). Add `SavePlaygroundSession` call when `req.SaveToDB == true`. Python client passes `save_to_db: true` in `CreatePlayground` RPC when the backtest should be persisted for analytics.

**Tagging convention for Metabase queries:** Simulator playgrounds need strategy-level tags to be queryable. The `Meta.Tags` (`pq.StringArray`) field already exists. Python should tag runs with `["simulator", "strategy:covered_call", "version:v7"]` before persisting.

---

## Docker Compose Addition

Add to `docker-compose.prod.yaml` (and local `observability/docker-compose.yaml` for dev testing):

```yaml
  metabase:
    image: metabase/metabase:v0.59.4
    environment:
      - MB_DB_TYPE=postgres
      - MB_DB_DBNAME=metabase
      - MB_DB_PORT=5432
      - MB_DB_HOST=postgres
      - MB_DB_USER=${METABASE_DB_USER}
      - MB_DB_PASS=${METABASE_DB_PASS}
      - MB_JETTY_PORT=3001
      - JAVA_OPTS=-Xmx768m
    ports:
      - "3001:3001"
    depends_on:
      - postgres
    volumes:
      - metabase-data:/metabase-data
    restart: unless-stopped
```

Add to `volumes:` block:
```yaml
  metabase-data:
```

Add to `.env.prod.template`:
```
METABASE_DB_USER=CHANGEME
METABASE_DB_PASS=CHANGEME
```

**Note:** `metabase-data` volume is used by Metabase for JAR plugin storage and local file caching, not for primary state (that goes to `MB_DB_*` Postgres). It is still useful to mount to avoid container-restart side effects.

---

## What Does NOT Need to Change

| Existing Component | Status | Reason |
|--------------------|--------|--------|
| Go GORM models | No change | Existing tables (`order_records`, `trade_records`, `playgrounds`, `equity_plot_records`) are the data source. Analytics views read them. |
| Python OTel instrumentation | No change | Metabase queries Postgres directly, not OTel pipelines. |
| Grafana / otel-lgtm | No change | Grafana stays on port 3000 for operational metrics. Metabase is for business analytics. They are additive, not overlapping. |
| `infra/init.sql` | Minor addition | Add `CREATE DATABASE metabase` and `CREATE SCHEMA analytics` + read-only user setup. |
| Postgres version (13) | No change | Postgres 13 is fully supported by Metabase v0.59. (Metabase supports all current Postgres versions.) |
| EventStoreDB | No change | Not involved in analytics at all. |

---

## Go Dependencies: None New

The simulator persistence change is a behavioral change to `database_service.go` — no new Go packages needed. GORM, uuid, and the existing Postgres driver are already present.

## Python Dependencies: None New

Python already calls `CreatePlayground` RPC. The only change is passing `save_to_db: true` in the protobuf request. No new Python packages.

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Business analytics | Metabase OSS | Grafana (extend existing) | Grafana requires writing PromQL/LogQL queries and raw JSON dashboards. Metabase gives non-engineers a GUI question builder. The project explicitly chose "business-level analytics" as a separate tool. |
| Metabase app DB | Existing Postgres (metabase DB) | Separate Postgres container | Overkill for a single-server deployment. One Postgres instance with two databases (playground + metabase) is the standard Metabase deployment pattern on constrained infrastructure. |
| Analytics layer | SQL views in Postgres | Separate dbt / transform pipeline | No dbt expertise in this codebase. Views are simpler, idempotent, and queryable by Metabase without additional tooling. |
| Multi-leg grouping | `tag` column convention | New `spread_group_id` column | `tag` already exists in `order_records` with JSONB `attributes` as a fallback. Adding a new column means a GORM migration; a tagging convention is zero-schema-change. |
| Metabase edition | OSS (`metabase/metabase`) | Enterprise (`metabase/metabase-enterprise`) | OSS includes all needed features (SQL editor, dashboards, PostgreSQL connection, AI SQL in v0.59). Enterprise adds SSO and audit logs — unnecessary for a single-operator trading platform. |

---

## Confidence Assessment

| Area | Confidence | Basis |
|------|------------|-------|
| Metabase v0.59.4 version | HIGH | Verified via GitHub releases page (latest release March 2025) |
| Metabase Docker image name | HIGH | Official Docker Hub: `metabase/metabase` |
| MB_JETTY_PORT for port change | HIGH | Official Metabase docs (Customizing Jetty Webserver) |
| MB_DB_* env vars for PostgreSQL app DB | HIGH | Official Metabase docs (Configuring Application Database) |
| Memory: ~600MB idle, ~1-1.5GB under load | MEDIUM | Metabase community forum posts (multiple sources agree); not official spec |
| Postgres 13 compatibility | HIGH | Metabase supports all current Postgres versions; PG13 is current |
| Simulator persistence: `SaveToDB` flag path | HIGH | Direct inspection of `database_service.go` CreatePlayground — simulator branch (line ~927) does NOT call SavePlaygroundSession; live/reconcile branches DO |
| Analytics views via `analytics` schema | HIGH | PostgreSQL schemas docs + GORM schema isolation pattern |
| `tag` column for spread grouping | HIGH | Direct inspection of `order_record.go` — `tag` column is `gorm:"column:tag;type:text"` |

---

## Sources

- [Metabase GitHub Releases](https://github.com/metabase/metabase/releases) — v0.59.4 latest stable (HIGH confidence)
- [Metabase Docker Hub](https://hub.docker.com/r/metabase/metabase/) — official image (HIGH confidence)
- [Metabase: Configuring Application Database](https://www.metabase.com/docs/latest/installation-and-operation/configuring-application-database) — MB_DB_* env vars (HIGH confidence)
- [Metabase: Customizing Jetty Webserver](https://www.metabase.com/docs/latest/configuring-metabase/customizing-jetty-webserver) — MB_JETTY_PORT (HIGH confidence)
- [Metabase: Running on Docker](https://www.metabase.com/docs/latest/installation-and-operation/running-metabase-on-docker) — Docker image usage (HIGH confidence)
- [Metabase: Memory requirements](https://discourse.metabase.com/t/what-are-metabase-minimum-resources/21470) — baseline ~600MB (MEDIUM confidence)
- [Metabase: How to run in production](https://www.metabase.com/learn/metabase-basics/administration/administration-and-operation/metabase-in-production) — production recommendations (HIGH confidence)
- [PostgreSQL: Schemas](https://www.postgresql.org/docs/current/ddl-schemas.html) — `analytics` schema isolation pattern (HIGH confidence)
- Existing codebase: `src/go/data/database_service.go` lines 821-971 — CreatePlayground simulator branch (HIGH confidence, direct inspection)
- Existing codebase: `src/go/backtester-api/models/order_record.go` — `tag` and `attributes` columns (HIGH confidence, direct inspection)
- Existing codebase: `docker-compose.prod.yaml` — current 4-service production stack (HIGH confidence, direct inspection)

---

*Stack research for: Metabase Analytics (v2.0) — additions to existing Go + Python + OTel + Grafana + PostgreSQL stack*
*Researched: 2026-03-29*
