# Phase 15: Simulator Persistence & Backtest Comparison - Research

**Researched:** 2026-03-30
**Domain:** Python CLI persistence, PostgreSQL schema extension, Metabase dashboard provisioning
**Confidence:** HIGH

## Summary

Phase 15 adds opt-in persistence of simulator backtest results and a comparison dashboard. The work spans three layers: (1) adding a `--save-to-db` flag to Python demo scripts that calls the existing `SavePlayground` RPC, (2) creating a `backtest_runs` summary table populated by the Python client after save, and (3) extending `provision-metabase.py` with a fourth "Strategy Comparison" dashboard.

The existing `SavePlayground` RPC already handles simulator playgrounds via `RemapAndSavePlayground` (id_remap.go) which remaps in-memory nonce IDs to fresh GORM auto-increment IDs. This means PERSIST-01 requires only adding the RPC call on the Python side -- no Go server changes. PERSIST-02 requires a new `backtest_runs` table and Python-side SQL insert (psycopg2 must be added to the environment). DASH-03 follows the established `provision-metabase.py` pattern with native SQL cards.

**Primary recommendation:** Add `psycopg2-binary` to the Python environment, use direct SQL INSERT for `backtest_runs` (simpler than adding a new RPC), and extend `analytics-schema.sql` with the table definition.

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Explicit `--save-to-db` flag on the Python client CLI -- only persist when flag is passed
- D-01: Calls existing `SavePlayground` RPC on completion -- no Go server save logic changes for PERSIST-01
- D-02: `client_id` as primary human-readable identifier (e.g. `mean-reversion-aapl-2026-03-30`)
- D-02: `backtest_runs` table columns: playground_id (FK), client_id, strategy_name, parameters (JSONB), final_balance, starting_balance, win_rate, profit_factor, total_pnl, total_trades, start_date, end_date, created_at
- D-02: Populated by the Python client after backtest completion (compute metrics from playground state, then insert)
- D-03: Table + overlay charts: sortable table of all backtest runs with key metrics
- D-03: Overlay equity curves for selected runs (multi-select playground filter)
- D-03: Filter by strategy type and date range
- D-03: Added to `infra/provision-metabase.py` following existing dashboard-as-code pattern
- D-04: JSONB column (`parameters`) in backtest_runs for strategy constructor args

### Claude's Discretion
- Whether backtest_runs is populated via a new RPC or Python-side SQL insert
- Exact schema for backtest_runs (additional columns beyond the decided ones)
- Dashboard layout and card arrangement for DASH-03
- How to serialize strategy parameters (which args to include/exclude)

### Deferred Ideas (OUT OF SCOPE)
- Automated backtest comparison reports -- future enhancement
- Backtest scheduling/automation -- out of scope
- Parameter optimization dashboard -- could be Phase 16+ addition

</user_constraints>

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PERSIST-01 | Simulator playgrounds persist order_records and trade_records to Postgres on completion | `SavePlayground` RPC exists in Go; `RemapAndSavePlayground` handles simulator ID remapping. Python client calls `SavePlayground` RPC after tick loop. Only Python-side change needed. |
| PERSIST-02 | `backtest_runs` summary table (final_balance, win_rate, profit_factor, parameters JSONB) | New table in analytics-schema.sql. Python computes metrics from playground state + `v_playground_stats` view, then INSERTs via psycopg2. |
| DASH-03 | Strategy comparison dashboard: compare backtests by parameters and strategy types | Extends `provision-metabase.py` with 4th dashboard. SQL queries against backtest_runs + equity_plot_records. Multi-select filter using Metabase template-tags. |

</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| psycopg2-binary | 2.9.x | Python Postgres driver for backtest_runs INSERT | Standard Python PostgreSQL adapter; binary variant avoids C compilation |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| requests | 2.33.0 (already installed) | Metabase API calls in provision script | Already used by provision-metabase.py |
| argparse | stdlib | CLI flag parsing | Already used in demo scripts |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| psycopg2 (Python INSERT) | New Go RPC `SaveBacktestRun` | RPC adds proto changes, Go handler, Python client code -- heavier for a simple INSERT; psycopg2 is simpler |
| psycopg2-binary | psycopg (v3) | psycopg v3 is newer but psycopg2 is more established; either works fine |

**Installation:**
```bash
/Users/jamal/miniconda3/envs/grodt/bin/pip install psycopg2-binary
```

**Recommendation on discretion item:** Use Python-side SQL INSERT via psycopg2 for `backtest_runs`. Rationale:
1. The table is write-once (one row per backtest run) -- no complex transaction needed
2. Avoids proto changes and Go server modifications
3. Python already has the computed metrics in-memory after the backtest
4. The Postgres connection string is already available via environment (`POSTGRES_HOST`, `POSTGRES_USER`, etc.)

## Architecture Patterns

### Recommended Project Structure
```
infra/
  analytics-schema.sql          # Extend with backtest_runs table + view
  provision-metabase.py         # Extend with Dashboard 4: Strategy Comparison
src/clients/python/
  engine/
    client.py                   # Add save_playground() method wrapping RPC call
    persistence.py              # NEW: backtest_runs INSERT + metrics computation
  demos/
    demo_mean_reversion.py      # Add --save-to-db flag
```

### Pattern 1: Save Flow After Backtest Completion
**What:** After the tick loop completes, optionally persist results to Postgres
**When to use:** When `--save-to-db` CLI flag is passed
**Example:**
```python
# In demo_mean_reversion.py main(), after run_strategy() returns:
if args.save_to_db:
    from rpc.playground_pb2 import SavePlaygroundRequest
    # 1. Call SavePlayground RPC (persists orders, trades, equity plots)
    request = SavePlaygroundRequest(playground_id=playground.id)
    playground.network_call_with_retry(
        'save_playground', playground.client.SavePlayground, request
    )

    # 2. Compute metrics and insert backtest_runs row
    from engine.persistence import save_backtest_run
    save_backtest_run(
        playground_id=playground.id,
        client_id=playground.client_id,
        strategy_name=strategy.label,
        parameters={...},  # strategy constructor args
        final_balance=playground.account.balance,
        starting_balance=args.balance,
        start_date=args.start,
        end_date=args.end,
    )
```

### Pattern 2: backtest_runs Table Design
**What:** Summary table for comparing backtest runs
**Schema:**
```sql
CREATE TABLE IF NOT EXISTS backtest_runs (
    id              SERIAL PRIMARY KEY,
    playground_id   UUID NOT NULL REFERENCES playground_sessions(id),
    client_id       TEXT NOT NULL,
    strategy_name   TEXT NOT NULL,
    parameters      JSONB NOT NULL DEFAULT '{}',
    starting_balance NUMERIC NOT NULL,
    final_balance   NUMERIC NOT NULL,
    total_pnl       NUMERIC NOT NULL,
    win_rate        NUMERIC,
    profit_factor   NUMERIC,
    total_trades    INTEGER NOT NULL DEFAULT 0,
    start_date      DATE,
    end_date        DATE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_backtest_runs_strategy
    ON backtest_runs (strategy_name);
CREATE INDEX IF NOT EXISTS idx_backtest_runs_playground
    ON backtest_runs (playground_id);
```

### Pattern 3: Metrics Computation in Python
**What:** Compute win_rate, profit_factor, total_trades from playground state
**When to use:** After SavePlayground RPC succeeds, before backtest_runs INSERT
**Approach:** Two options:
1. **Compute from v_playground_stats view** (query Postgres after save) -- authoritative, matches dashboard data
2. **Compute from Python-side state** (playground.account, orders) -- faster, no extra query

**Recommendation:** Query `v_playground_stats` after the SavePlayground RPC. This ensures metrics are consistent with what the existing dashboards show, and the view is already validated (Phase 13).

```python
# After SavePlayground succeeds:
cursor.execute("""
    SELECT total_trades, total_pnl, win_rate, profit_factor
    FROM v_playground_stats
    WHERE playground_id = %s
""", (playground_id,))
stats = cursor.fetchone()
```

### Pattern 4: Parameter Serialization
**What:** Capture strategy constructor args as JSONB
**Recommendation:** Each strategy should expose a `get_parameters()` method returning a dict of its tuning knobs. For MeanReversionStrategy this would include:
- `max_loss_pct`, `stop_percentile`, `total_shares_per_group`, `num_exit_tiers`
- `htf_horizon`, `tier_spacing`, `stop_widen_on_exit`, `min_expected_profit`, `ev_model`, `model` (return model type)

Exclude non-parameter args (playground, symbol, logger, pdf object).

### Pattern 5: Metabase Dashboard with Multi-Select
**What:** DASH-03 comparison dashboard with multi-select playground filter
**Key insight:** Existing dashboards use a single playground_id filter. The comparison dashboard needs a multi-select filter OR a strategy_name + date range filter. Use Metabase template-tags with `type: text` for strategy_name and playground_id list.

**Dashboard cards:**
1. **Backtest Runs Table** -- all runs sortable by metrics (table display)
2. **Overlay Equity Curves** -- equity_plot_records for multiple playground_ids (line chart)
3. **Parameter Comparison** -- JSONB parameter values for selected runs (table)
4. **Best/Worst Metrics** -- scalar cards for top/bottom performers

### Anti-Patterns to Avoid
- **Computing metrics differently than v_playground_stats:** Would create inconsistencies between the comparison dashboard and per-playground dashboard
- **Auto-saving all backtests:** The explicit flag design (D-01) avoids polluting the DB during rapid iteration
- **Storing parameters as separate columns:** JSONB is the right choice (D-04) -- new strategies don't need schema changes

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Postgres connection management | Raw socket/connection code | psycopg2 connection with env vars | Handles encoding, escaping, connection pooling |
| Win rate / profit factor computation | Manual Python calculation | `v_playground_stats` SQL view (Phase 13) | Already validated, authoritative source |
| ID remapping for simulator orders | Custom ID assignment logic | Existing `RemapAndSavePlayground` in Go | Already handles nonce-to-GORM ID remapping |
| Metabase card creation | Direct API calls | `upsert_card()` / `upsert_dashboard()` helpers | Already handles create-or-update idempotently |

## Common Pitfalls

### Pitfall 1: Simulator ID Remapping Race Condition
**What goes wrong:** Calling SavePlayground on a simulator playground that has already been saved could cause duplicate IDs
**Why it happens:** `RemapAndSavePlayground` creates new GORM rows -- calling it twice duplicates data
**How to avoid:** Only call SavePlayground once per backtest run; check if playground_id already exists in playground_sessions before insert
**Warning signs:** Duplicate order_records for the same logical playground

### Pitfall 2: Playground ID Changes After Save
**What goes wrong:** `RemapAndSavePlayground` may assign a NEW UUID to the playground_sessions record (the in-memory playground UUID was the GORM model ID, which gets a new value on Create)
**Why it happens:** The simulator playground uses `uuid.New()` at creation time, but GORM `Save` on a new record may behave as Create if the record doesn't exist yet
**How to avoid:** Read the actual Go code -- `RemapAndSavePlayground` does `tx.Save(playground)` which preserves the UUID since it's the primary key. The UUID field is set before creation and uses `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"` -- so GORM uses the existing UUID, not generating a new one. The remapping is only for order/trade `uint` IDs (auto-increment). **This pitfall does NOT apply** -- the playground UUID is preserved.
**Warning signs:** N/A -- confirmed safe by code review

### Pitfall 3: psycopg2 Not Available in Conda Env
**What goes wrong:** ImportError when trying to insert backtest_runs
**Why it happens:** psycopg2 is not currently in the grodt conda env or requirements.txt
**How to avoid:** Add `psycopg2-binary` to requirements.txt and install before running
**Warning signs:** ModuleNotFoundError: No module named 'psycopg2'

### Pitfall 4: Metabase Multi-Select Filtering
**What goes wrong:** Overlay equity curves query needs to handle multiple playground_ids but Metabase template-tags are single-value by default
**Why it happens:** Existing dashboards use `WHERE playground_id = {{playground_id}}::uuid` (single value)
**How to avoid:** For the comparison dashboard, filter by `strategy_name` and `date range` from `backtest_runs` table. Or use Metabase's field filter type for multi-select. The simplest approach: query `backtest_runs` with strategy_name filter, then join to equity_plot_records.
**Warning signs:** Dashboard shows only one run at a time instead of overlaid curves

### Pitfall 5: v_playground_stats Returns NULL After Save
**What goes wrong:** Querying v_playground_stats immediately after SavePlayground returns empty results
**Why it happens:** The view queries playground_sessions, order_records, trade_records -- all need to be committed
**How to avoid:** The SavePlayground RPC uses a transaction, so all data is committed atomically. The Python client should query AFTER the RPC returns successfully.
**Warning signs:** `stats = cursor.fetchone()` returns None

### Pitfall 6: Equity Plot Records Not Available for Simulators
**What goes wrong:** Equity curve overlay shows no data for simulator backtests
**Why it happens:** Need to verify that equity_plot_records are actually saved during SavePlayground for simulators
**How to avoid:** Confirmed in code: `RemapAndSavePlayground` (id_remap.go) calls `saveEquityPlots` which persists equity plot records. The `SavePlayground` method in database_service.go also calls `saveEquityPlotRecords`. This is safe.
**Warning signs:** Empty equity curve chart for saved backtests

## Code Examples

### Existing SavePlayground RPC Call Pattern
```python
# Source: src/clients/python/rpc/playground_twirp.py (line 281)
# The Python Twirp client already has SavePlayground:
from rpc.playground_pb2 import SavePlaygroundRequest

request = SavePlaygroundRequest(playground_id=playground.id)
response = playground.network_call_with_retry(
    'save_playground',
    playground.client.SavePlayground,
    request
)
```

### Existing Demo Script CLI Pattern
```python
# Source: src/clients/python/demos/demo_mean_reversion.py
parser.add_argument("--save-to-db", action="store_true",
    help="Persist backtest results to Postgres on completion")
```

### Existing Provision Script Card Pattern
```python
# Source: infra/provision-metabase.py
cards["runs_table"] = upsert_card(session, base_url, _make_card(
    "Comparison: Backtest Runs", db_id,
    """SELECT br.client_id, br.strategy_name, br.starting_balance,
           br.final_balance, br.total_pnl, br.win_rate,
           br.profit_factor, br.total_trades,
           br.start_date, br.end_date, br.created_at
    FROM backtest_runs br
    WHERE ({{strategy_name}} = '' OR br.strategy_name = {{strategy_name}})
    ORDER BY br.created_at DESC""",
    display="table",
))
```

### Postgres Connection from Python (Environment Variables)
```python
# Source: existing pattern from cmd/run-dev.sh + .env
import os
import psycopg2

conn = psycopg2.connect(
    host=os.getenv("POSTGRES_HOST", "localhost"),
    port=int(os.getenv("POSTGRES_PORT", "5432")),
    dbname=os.getenv("POSTGRES_DB", "playground"),
    user=os.getenv("POSTGRES_USER", "grodt"),
    password=os.getenv("POSTGRES_PASSWORD", "test747"),
)
```

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | pytest (Python) + go test (Go) |
| Config file | None for pytest; taskfile.yml for go test |
| Quick run command | `cd src/clients/python && /Users/jamal/miniconda3/envs/grodt/bin/python -m pytest tests/test_persistence.py -x` |
| Full suite command | `cd src/clients/python && /Users/jamal/miniconda3/envs/grodt/bin/python -m pytest tests/ -x` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PERSIST-01 | SavePlayground RPC called when --save-to-db flag present | unit (mock RPC) | `pytest tests/test_persistence.py::test_save_to_db_calls_rpc -x` | Wave 0 |
| PERSIST-02 | backtest_runs row inserted with correct metrics | unit (mock DB) | `pytest tests/test_persistence.py::test_backtest_run_inserted -x` | Wave 0 |
| PERSIST-02 | parameters JSONB contains strategy constructor args | unit | `pytest tests/test_persistence.py::test_parameters_serialization -x` | Wave 0 |
| DASH-03 | provision script creates comparison dashboard | manual-only | Run `provision-metabase.py` against test Metabase | N/A (manual) |

### Sampling Rate
- **Per task commit:** `cd src/clients/python && /Users/jamal/miniconda3/envs/grodt/bin/python -m pytest tests/test_persistence.py -x`
- **Per wave merge:** `cd src/clients/python && /Users/jamal/miniconda3/envs/grodt/bin/python -m pytest tests/ -x`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `tests/test_persistence.py` -- covers PERSIST-01, PERSIST-02
- [ ] `psycopg2-binary` install: `/Users/jamal/miniconda3/envs/grodt/bin/pip install psycopg2-binary`

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| PostgreSQL | backtest_runs INSERT | Yes (via Docker/port-forward) | 13 | -- |
| psycopg2-binary | Python DB driver | No | -- | Install via pip (Wave 0 task) |
| Metabase | DASH-03 dashboard | Yes (Windows Desktop, port 3001) | 0.59.4 | -- |
| Go server | SavePlayground RPC | Yes (task app:dev) | -- | -- |
| Python grodt env | CLI scripts | Yes | 3.10 | -- |

**Missing dependencies with no fallback:**
- None (psycopg2-binary is easily installable)

**Missing dependencies with fallback:**
- psycopg2-binary: Not installed, must be added (trivial `pip install`)

## Open Questions

1. **Postgres environment variables on Python side**
   - What we know: Go server uses `POSTGRES_HOST`, `POSTGRES_USER`, etc. from `.env` loaded via godotenv
   - What's unclear: Whether the Python client has access to these same env vars (they're set in `.env` which godotenv loads for Go, but Python scripts may not source it)
   - Recommendation: Use `python-dotenv` (already a pattern) or read from `.env` directly. Or hardcode defaults matching the dev setup (`localhost:5432/playground`, `grodt/test747`).

2. **Strategy parameter method on BaseStrategy**
   - What we know: MeanReversionStrategy has specific tuning knobs
   - What's unclear: Whether to add `get_parameters()` to BaseStrategy as abstract or just implement on each strategy
   - Recommendation: Add a concrete `get_parameters()` method to BaseStrategy that returns `{}` by default. Strategies override to include their specific params. This avoids breaking existing strategies.

## Sources

### Primary (HIGH confidence)
- `src/go/backtester-api/router/grpc.go:674-690` -- SavePlayground RPC handler
- `src/go/data/database_service.go:1603-1637` -- SavePlayground with RemapAndSavePlayground for simulators
- `src/go/backtester-api/models/id_remap.go:34` -- RemapAndSavePlayground implementation
- `src/clients/python/engine/client.py` -- BacktesterPlaygroundClient with client_id, network_call_with_retry
- `src/clients/python/demos/demo_mean_reversion.py` -- CLI argparse pattern, strategy construction
- `src/clients/python/engine/trading_engine.py` -- run_strategy() tick loop, strategy.label usage
- `infra/analytics-schema.sql` -- v_playground_stats view definition
- `infra/provision-metabase.py` -- upsert_card, upsert_dashboard, _make_card patterns

### Secondary (MEDIUM confidence)
- `src/clients/python/rpc/playground_twirp.py:281` -- Python SavePlayground client method

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- psycopg2-binary is well-established, no Go changes needed
- Architecture: HIGH -- all integration points verified in source code
- Pitfalls: HIGH -- each pitfall verified by reading actual implementation code

**Research date:** 2026-03-30
**Valid until:** 2026-04-30 (stable domain, no fast-moving dependencies)
