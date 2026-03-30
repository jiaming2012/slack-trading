# Phase 14: Core Performance Dashboards - Research

**Researched:** 2026-03-30
**Domain:** Metabase REST API dashboard provisioning, SQL analytics views, Python scripting
**Confidence:** HIGH

## Summary

Phase 14 builds 3 Metabase dashboards (Trading Performance, Slippage Analysis, Portfolio Analytics) programmatically via a Python provisioning script that calls the Metabase REST API. The data layer is already complete -- 4 SQL views (v_playground_stats, v_order_pnl, v_trade_fills, v_open_slippage) deployed in Phase 13, Metabase running at 192.168.8.164:3001 with playground DB connected via metabase_ro.

The primary technical challenge is authoring the `infra/provision-metabase.py` script that creates native SQL questions, assembles them into dashboards with shared filter parameters, and is re-runnable (idempotent). The Metabase API supports all required operations: `POST /api/card` for questions, `POST /api/dashboard` for dashboards, and `PUT /api/dashboard/:id/cards` for arranging cards with positioning and parameter mappings.

**Primary recommendation:** Use native SQL questions (not MBQL) for all dashboard cards -- this gives full control over SQL, matches the existing views, and avoids MBQL version compatibility issues. Use `{{playground_id}}` template-tag syntax in SQL for filter variables, then map them to dashboard-level filter parameters.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: 3 separate dashboards (Trading Performance, Slippage Analysis, Portfolio Analytics)
- D-02: Playground selector uses client_id primary, playground_id fallback, environment + tags filters, multi-select
- D-03: Dashboard-as-code via Python script `infra/provision-metabase.py` using Metabase REST API
- D-04: Equity curve as line chart with drawdown overlay + starting balance reference line
- D-05: SQL views abstract raw schema; dashboards query views not tables; analytics-schema.sql idempotent

### Claude's Discretion
- Exact Metabase question types (native SQL vs simple questions vs custom questions)
- Chart types for non-equity visualizations (bar, table, number cards)
- Dashboard layout and card sizing
- Filter widget types (dropdown vs search vs multi-select)

### Deferred Ideas (OUT OF SCOPE)
- Strategy comparison dashboard (DASH-03) -- belongs in Phase 15
- Spread analytics dashboard (DASH-05) -- belongs in Phase 16
- Per-asset-class split in v_playground_stats -- can be addressed via GROUP BY class in dashboard queries
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DASH-01 | Trading performance dashboard: P&L over time, win rate, profit factor, gross profit/loss | v_playground_stats provides all aggregate metrics; v_order_pnl provides per-order P&L for time series; equity_plot_records for equity curve |
| DASH-02 | Slippage analysis dashboard: open/close/total slippage per playground | v_open_slippage covers open side; close slippage requires new SQL query or view extension (see Pitfall 1) |
| DASH-04 | Portfolio analytics: position history, per-symbol/asset-class breakdown | v_trade_fills provides symbol/class breakdown; v_order_pnl with GROUP BY symbol for per-symbol P&L |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| requests | 2.33.0 | HTTP client for Metabase API | Already in grodt conda env, verified |
| Metabase REST API | v0.59.4 | Dashboard/card/parameter CRUD | Already deployed at 192.168.8.164:3001 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| json (stdlib) | - | Serialize API payloads | All API calls |
| argparse (stdlib) | - | CLI args for Metabase URL/credentials | Script entry point |
| os/sys (stdlib) | - | Environment variable access | Credential management |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Raw requests | metabase_api_python (PyPI) | Adds dependency; raw requests is simpler for this use case and avoids version mismatch risks |
| Native SQL questions | MBQL queries | MBQL is harder to write/debug, version-dependent (MBQL 4 vs 5), and SQL views already exist |

**Installation:**
No new packages needed. `requests` 2.33.0 already available in grodt conda env.

## Architecture Patterns

### Recommended Project Structure
```
infra/
  analytics-schema.sql          # SQL views (Phase 13, exists)
  init-metabase.sql             # DB/role setup (Phase 12, exists)
  provision-metabase.py         # NEW: Dashboard provisioning script
```

### Pattern 1: Metabase API Provisioning Workflow
**What:** Python script that creates questions and dashboards via REST API in a deterministic order
**When to use:** Any time dashboards need to be created or updated programmatically

**Workflow:**
1. Authenticate: `POST /api/session` with email/password to get session token
2. Get database ID: `GET /api/database` to find the playground database
3. Create questions: `POST /api/card` for each native SQL question
4. Create dashboards: `POST /api/dashboard` with parameter definitions
5. Add cards to dashboards: `PUT /api/dashboard/:id/cards` with positioning and parameter_mappings

**Example -- Create a native SQL question:**
```python
# Source: Metabase API docs + community examples
card_payload = {
    "name": "Total P&L by Playground",
    "dataset_query": {
        "database": DB_ID,
        "type": "native",
        "native": {
            "query": """
                SELECT playground_id, total_pnl, win_rate, profit_factor,
                       gross_profit, gross_loss, total_trades
                FROM v_playground_stats
                WHERE playground_id = {{playground_id}}
            """,
            "template-tags": {
                "playground_id": {
                    "id": "pg_id_tag",
                    "name": "playground_id",
                    "display-name": "Playground ID",
                    "type": "text"
                }
            }
        }
    },
    "display": "table",
    "visualization_settings": {}
}
resp = session.post(f"{MB_URL}/api/card", json=card_payload)
card_id = resp.json()["id"]
```

**Example -- Create a dashboard with filters:**
```python
dashboard_payload = {
    "name": "Trading Performance",
    "parameters": [
        {
            "id": "playground_filter",
            "name": "Playground",
            "slug": "playground",
            "type": "string/=",
            "sectionId": "string"
        }
    ]
}
resp = session.post(f"{MB_URL}/api/dashboard", json=dashboard_payload)
dashboard_id = resp.json()["id"]
```

**Example -- Add cards to dashboard with parameter mapping:**
```python
cards_payload = {
    "cards": [
        {
            "card_id": card_id,
            "row": 0,
            "col": 0,
            "size_x": 6,
            "size_y": 4,
            "parameter_mappings": [
                {
                    "parameter_id": "playground_filter",
                    "card_id": card_id,
                    "target": ["variable", ["template-tag", "playground_id"]]
                }
            ]
        }
    ]
}
session.put(f"{MB_URL}/api/dashboard/{dashboard_id}/cards", json=cards_payload)
```

### Pattern 2: Idempotent Provisioning (Re-runnable Script)
**What:** Script detects existing resources and updates rather than duplicating
**When to use:** Every run of provision-metabase.py

**Approach:**
1. Before creating a card, check `GET /api/card` and search by name
2. If found, use `PUT /api/card/:id` to update; if not, `POST /api/card` to create
3. Same for dashboards: check `GET /api/dashboard` by name
4. For dashboard cards: `PUT /api/dashboard/:id/cards` replaces all cards (not additive)

```python
def find_or_create_card(session, name, payload):
    """Idempotent card creation -- update if exists, create if not."""
    existing = session.get(f"{MB_URL}/api/card").json()
    match = next((c for c in existing if c["name"] == name), None)
    if match:
        session.put(f"{MB_URL}/api/card/{match['id']}", json=payload)
        return match["id"]
    else:
        resp = session.post(f"{MB_URL}/api/card", json=payload)
        return resp.json()["id"]
```

### Pattern 3: Playground Selector with client_id + Fallback
**What:** SQL COALESCE pattern for human-readable playground identification
**When to use:** All dashboard filter queries

```sql
-- Used in every dashboard question that needs playground identification
SELECT
    p.id AS playground_id,
    COALESCE(p.client_id, p.id::text) AS display_name,
    p.environment,
    p.tags
FROM playground_sessions p
```

This provides the dropdown values. The filter uses `display_name` for the user but passes `playground_id` to all questions.

### Pattern 4: Equity Curve with Drawdown Overlay
**What:** Two-series line chart: equity line + drawdown shading
**When to use:** DASH-01 equity visualization

```sql
-- Equity curve with running max and drawdown
SELECT
    e.timestamp,
    e.equity,
    MAX(e.equity) OVER (ORDER BY e.timestamp) AS peak_equity,
    e.equity - MAX(e.equity) OVER (ORDER BY e.timestamp) AS drawdown,
    p.starting_balance
FROM equity_plot_records e
JOIN playground_sessions p ON p.id = e.playground_session_id
WHERE e.playground_session_id = {{playground_id}}
ORDER BY e.timestamp
```

In Metabase, this renders as a line chart with `equity` as the primary line and `drawdown` as a secondary area/bar series. The `starting_balance` can be a constant reference line via visualization_settings.

### Anti-Patterns to Avoid
- **Building MBQL queries programmatically:** MBQL is complex, version-dependent, and underdocumented for programmatic use. Native SQL is clearer and already has views ready.
- **Creating cards inline on dashboards (virtual cards):** Virtual/text cards are fine for headers, but data cards should be saved questions for reusability and debugging.
- **Hardcoding database IDs:** Always look up the database ID dynamically via `GET /api/database`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SQL P&L aggregation | Custom Python calculations | v_playground_stats view | Already mirrors CalcRealizedPL(), tested in Phase 13 |
| Per-order P&L | Order-level profit calculation | v_order_pnl view | Handles 4 order side cases correctly |
| Slippage calculation | Custom slippage math | v_open_slippage view + close slippage query | Consistent with playground_metrics.py |
| Dashboard CRUD | Manual Metabase UI clicks | Metabase REST API via Python | Reproducible, version-controlled |
| UUID display | Custom formatting | COALESCE(client_id, id::text) | Metabase renders this naturally |

## Common Pitfalls

### Pitfall 1: Missing Close Slippage View
**What goes wrong:** DASH-02 requires open/close/total slippage, but only v_open_slippage exists (covers buy/buy_to_open, sell_short/sell_to_open sides only)
**Why it happens:** Phase 13 created open-side slippage only; close-side slippage (sell/sell_to_close, buy_to_cover/buy_to_close) was not in scope
**How to avoid:** Add a close slippage query in the provisioning script's native SQL, or extend analytics-schema.sql with a v_close_slippage view. The SQL pattern mirrors v_open_slippage but with reversed sign logic:
```sql
-- Close-side slippage (not in analytics-schema.sql yet)
SELECT
    o.id AS order_id, o.playground_id, o.symbol, o.class, o.side,
    o.requested_price, t.price AS fill_price, t.quantity AS fill_qty,
    CASE
        WHEN o.side IN ('sell', 'sell_to_close') THEN o.requested_price - t.price
        WHEN o.side IN ('buy_to_cover', 'buy_to_close') THEN t.price - o.requested_price
        ELSE 0
    END AS slippage_points
FROM order_records o
JOIN trade_records t ON t.order_id = o.id
WHERE o.deleted_at IS NULL AND t.deleted_at IS NULL
  AND o.status = 'filled'
  AND o.side IN ('sell', 'sell_to_close', 'buy_to_cover', 'buy_to_close')
```
**Warning signs:** Dashboard shows open slippage but no close slippage columns

### Pitfall 2: Metabase Session Token Expiry
**What goes wrong:** Long-running provisioning script gets 401 errors mid-execution
**Why it happens:** Metabase session tokens expire (default 2 weeks, but can vary)
**How to avoid:** Use API key auth (`x-api-key` header) instead of session auth if available, or create session at script start (provisioning runs in seconds, not hours). Alternatively, generate an API key via Metabase admin settings.
**Warning signs:** 401 responses after initial success

### Pitfall 3: Template Tag ID Collisions
**What goes wrong:** Dashboard filter doesn't connect to question variables
**Why it happens:** The `parameter_mappings` target must exactly match the template-tag name in the question's native query
**How to avoid:** Use consistent template-tag names across all questions (e.g., always `playground_id` for the playground filter). The `target` in parameter_mappings must be `["variable", ["template-tag", "playground_id"]]`.
**Warning signs:** Filter widget appears but selecting a value doesn't change results

### Pitfall 4: Card Positioning Overlap
**What goes wrong:** Dashboard cards overlap or stack incorrectly
**Why it happens:** `PUT /api/dashboard/:id/cards` uses a grid system (18 columns wide); cards need explicit row/col/size_x/size_y
**How to avoid:** Plan card layout on paper first. Standard Metabase grid is 18 columns. Common card sizes: number card (6x3), table (12x6), chart (9x6), full-width chart (18x6).
**Warning signs:** Cards visually overlapping in the dashboard

### Pitfall 5: playground_metrics.py Uses VWAP But SQL Uses AVG
**What goes wrong:** Validation shows small numeric differences between dashboard and playground_metrics.py
**Why it happens:** playground_metrics.py uses `_calc_trade_position` which computes true VWAP (sum(price*qty)/sum(qty)), but the SQL views use simple AVG(price) to match Go's CalcRealizedPL(). The Python script cross-validates internally (line 190: `assert abs(pl_1 - pl_2) < 0.001`), confirming the difference is within tolerance.
**How to avoid:** Compare dashboard numbers against `order.pl` field (which comes from Go CalcRealizedPL), not against Python's manual VWAP calculation. Phase 13 already established this -- "rounding tolerance" in success criteria accounts for this.
**Warning signs:** Small differences (< $0.01 per trade) between dashboard and Python output

### Pitfall 6: Multi-Select Filter with Native SQL
**What goes wrong:** Multi-select playground filter doesn't work with `WHERE playground_id = {{playground_id}}`
**Why it happens:** Multi-select requires `WHERE playground_id IN ({{playground_id}})` or field filter syntax
**How to avoid:** For multi-select support, use Metabase field filters (type: "dimension" mapped to the actual column) instead of basic text template-tags. Field filters automatically handle multi-select. Alternatively, use `WHERE {{playground_id}}` with a field filter variable type mapped to `playground_sessions.id`.
**Warning signs:** Selecting multiple playgrounds returns empty results

## Code Examples

### Full Provisioning Script Skeleton
```python
#!/usr/bin/env python3
"""Provision Metabase dashboards for trading analytics.

Usage:
    python infra/provision-metabase.py --url http://192.168.8.164:3001 --user jac475@cornell.edu --password <pw>

Re-runnable: updates existing dashboards/cards, creates missing ones.
"""
import argparse
import requests
import sys

def authenticate(base_url, email, password):
    resp = requests.post(f"{base_url}/api/session",
                         json={"username": email, "password": password})
    resp.raise_for_status()
    return resp.json()["id"]  # session token

def get_database_id(session, base_url, db_name="playground"):
    dbs = session.get(f"{base_url}/api/database").json()["data"]
    match = next((d for d in dbs if d["name"] == db_name), None)
    if not match:
        raise RuntimeError(f"Database '{db_name}' not found in Metabase")
    return match["id"]

def find_card_by_name(session, base_url, name):
    cards = session.get(f"{base_url}/api/card").json()
    return next((c for c in cards if c["name"] == name), None)

def find_dashboard_by_name(session, base_url, name):
    dashboards = session.get(f"{base_url}/api/dashboard").json()
    return next((d for d in dashboards if d["name"] == name), None)

def upsert_card(session, base_url, payload):
    existing = find_card_by_name(session, base_url, payload["name"])
    if existing:
        resp = session.put(f"{base_url}/api/card/{existing['id']}", json=payload)
        resp.raise_for_status()
        return existing["id"]
    resp = session.post(f"{base_url}/api/card", json=payload)
    resp.raise_for_status()
    return resp.json()["id"]
```

### Equity Curve Query (DASH-01)
```sql
-- Source: equity_plot_records + playground_sessions
SELECT
    e.timestamp,
    e.equity,
    MAX(e.equity) OVER (ORDER BY e.timestamp) AS peak_equity,
    e.equity - MAX(e.equity) OVER (ORDER BY e.timestamp) AS drawdown,
    p.starting_balance
FROM equity_plot_records e
JOIN playground_sessions p ON p.id = e.playground_session_id
WHERE e.playground_session_id = {{playground_id}}
ORDER BY e.timestamp
```

### Per-Symbol P&L Breakdown (DASH-04)
```sql
-- Source: v_order_pnl
SELECT
    symbol,
    class,
    COUNT(*) AS trades,
    SUM(realized_pl) AS total_pnl,
    COUNT(*) FILTER (WHERE realized_pl > 0) AS winners,
    COUNT(*) FILTER (WHERE realized_pl < 0) AS losers,
    SUM(realized_pl) FILTER (WHERE realized_pl > 0) AS gross_profit,
    SUM(realized_pl) FILTER (WHERE realized_pl < 0) AS gross_loss
FROM v_order_pnl
WHERE playground_id = {{playground_id}}
  AND side IN ('buy', 'buy_to_open', 'sell_short', 'sell_to_open')
GROUP BY symbol, class
ORDER BY total_pnl DESC
```

### Combined Slippage (Open + Close) Query (DASH-02)
```sql
-- Combines open and close slippage in one query
SELECT
    o.id AS order_id,
    o.playground_id,
    o.symbol,
    o.side,
    o.requested_price,
    t.price AS fill_price,
    t.quantity AS fill_qty,
    CASE
        WHEN o.side IN ('buy', 'buy_to_open') THEN t.price - o.requested_price
        WHEN o.side IN ('sell_short', 'sell_to_open') THEN o.requested_price - t.price
        WHEN o.side IN ('sell', 'sell_to_close') THEN o.requested_price - t.price
        WHEN o.side IN ('buy_to_cover', 'buy_to_close') THEN t.price - o.requested_price
        ELSE 0
    END AS slippage_points,
    CASE
        WHEN o.side IN ('buy', 'buy_to_open', 'sell_short', 'sell_to_open') THEN 'open'
        ELSE 'close'
    END AS slippage_type
FROM order_records o
JOIN trade_records t ON t.order_id = o.id
WHERE o.deleted_at IS NULL AND t.deleted_at IS NULL
  AND o.status = 'filled'
  AND o.playground_id = {{playground_id}}
ORDER BY o.timestamp
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| MBQL query format (v4) | MBQL 5 in API responses | Metabase 0.50+ | Use `?legacy-mbql=true` param if reading back MBQL; irrelevant for native SQL |
| Session-only auth | API key auth (`x-api-key`) | Metabase 0.49+ | API keys preferred for automation scripts |
| Dashboard cards via POST | `PUT /api/dashboard/:id/cards` replaces all cards | Metabase 0.49+ | PUT is the correct endpoint for setting dashboard card layout |

## Open Questions

1. **API Key vs Session Auth**
   - What we know: Metabase 0.49+ supports API keys. Instance is v0.59.4.
   - What's unclear: Whether an API key has already been generated for the admin user
   - Recommendation: Script should support both; prefer API key if available, fall back to session auth

2. **Multi-select implementation**
   - What we know: D-02 requires multi-select for playground filter. Field filters support multi-select natively.
   - What's unclear: Whether field filters work with SQL views (vs direct tables) in Metabase 0.59.4
   - Recommendation: Try field filter first; fall back to text template-tag with comma-separated parsing if needed

3. **Equity Plot Record Timing**
   - What we know: equity_plot_records exist in the schema. STATE.md notes "[Research]: Verify equity_plot_records write timing"
   - What's unclear: Whether live playgrounds write equity records incrementally or only on completion
   - Recommendation: Query will work either way; equity curve may show partial data for in-progress playgrounds (acceptable)

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Manual validation + Python script |
| Config file | none -- provisioning script validates via API responses |
| Quick run command | `python infra/provision-metabase.py --url http://192.168.8.164:3001 --user <email> --password <pw>` |
| Full suite command | Same + visual dashboard inspection |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DASH-01 | Trading Performance dashboard shows P&L, win rate, profit factor, equity curve | manual | Visual check: select playground, verify numbers match v_playground_stats query | N/A |
| DASH-02 | Slippage Analysis dashboard shows open/close/total slippage | manual | Visual check: select playground, verify slippage values | N/A |
| DASH-04 | Portfolio Analytics shows position history and per-symbol P&L | manual | Visual check: select playground, verify per-symbol breakdown | N/A |
| CROSS-01 | Dashboard numbers match playground_metrics.py within rounding tolerance | smoke | `python src/clients/python/tools/playground_metrics.py --playground-id <id>` and compare | Exists |

### Sampling Rate
- **Per task commit:** Run provisioning script, verify API returns 200 for all operations
- **Per wave merge:** Visual inspection of all 3 dashboards with a known playground
- **Phase gate:** Compare dashboard metrics vs playground_metrics.py output for at least 1 playground

### Wave 0 Gaps
- None -- this phase creates new files only (infra/provision-metabase.py). No test framework setup needed.

## Project Constraints (from CLAUDE.md)

- Python scripts use grodt conda env (`/Users/jamal/miniconda3/envs/grodt/bin/python`)
- Python files use `snake_case.py` naming
- Infra scripts go in `infra/` directory (established pattern)
- `gh` alias warning: use `/usr/local/bin/gh` for GitHub CLI
- No linter/formatter config -- follow existing code style
- GSD workflow enforcement for all edits

## Sources

### Primary (HIGH confidence)
- `infra/analytics-schema.sql` -- SQL views, indexes, table structure (local file)
- `src/clients/python/tools/playground_metrics.py` -- Reference P&L/slippage calculations (local file)
- `src/go/backtester-api/models/playground.go` -- Playground schema with client_id, environment, tags fields (local file)
- `src/go/backtester-api/models/playground_meta.go` -- Meta fields including starting_balance, environment, tags (local file)
- `src/go/backtester-api/models/equity_plot_record.go` -- Equity curve data model (local file)
- `.planning/phases/13-analytics-schema-indexes/13-01-SUMMARY.md` -- Phase 13 outputs (local file)
- `.planning/phases/12-deploy-metabase-harden-infrastructure/12-01-SUMMARY.md` -- Metabase infra setup (local file)

### Secondary (MEDIUM confidence)
- [Metabase API documentation](https://www.metabase.com/docs/latest/api) -- Official API reference
- [Working with the Metabase API](https://www.metabase.com/learn/metabase-basics/administration/administration-and-operation/metabase-api) -- Card/dashboard creation workflow
- [Adding dashboard tabs programmatically](https://discourse.metabase.com/t/how-to-add-dashboard-tabs-programmatically-in-metabase-0-49-which-endpoints-to-use/255084) -- PUT cards endpoint with tabs and positioning
- [Programmatically add cards to dashboard](https://discourse.metabase.com/t/metabase-rest-api-programmatically-add-cards-questions-to-dashboard/3301) -- Card creation and dashboard assembly

### Tertiary (LOW confidence)
- [SQL parameters documentation](https://www.metabase.com/docs/latest/questions/native-editor/sql-parameters) -- Template-tag syntax (not fetched directly, referenced via search)
- [Field filters documentation](https://www.metabase.com/docs/latest/questions/native-editor/field-filters) -- Field filter behavior for multi-select (not fetched directly)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new dependencies, all tools verified locally
- Architecture: HIGH -- Metabase API patterns well-documented, SQL views already tested
- Pitfalls: HIGH -- close slippage gap identified from code review, template-tag mapping from community docs

**Research date:** 2026-03-30
**Valid until:** 2026-04-30 (stable -- Metabase API rarely changes between minor versions)
