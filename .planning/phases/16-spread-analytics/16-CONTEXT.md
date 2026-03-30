# Phase 16: Spread Analytics - Context

**Gathered:** 2026-03-30
**Status:** Ready for planning

<domain>
## Phase Boundary

Group multi-leg option strategies as single trade units with combined P&L. The Go server auto-generates spread_group_key on PlaceMultiLegOrder, SQL views aggregate per-spread P&L, and a Metabase dashboard visualizes spread performance. Covered calls are NOT spreads — they're sequential and excluded from this phase.

Requirements: SCHEMA-03, SCHEMA-04, SPREAD-01, SPREAD-02, DASH-05

</domain>

<decisions>
## Implementation Decisions

### D-01: Spread grouping model
- **Attributes JSONB column** on order_records — store `spread_group_key` and `leg_role` in the existing `attributes` JSONB field
- No new tables needed (SCHEMA-03 is satisfied by the JSONB approach + a SQL view, not separate tables)
- SQL view groups orders by `attributes->>'spread_group_key'` for aggregation

### D-02: Spread group key generation — centralized on Go server
- **Go server generates** the spread_group_key when processing `PlaceMultiLegOrder` RPC
- All legs in the same multi-leg request automatically get the same spread_group_key
- Key format: UUID or descriptive string (Claude's discretion)
- Each leg also gets a `leg_role` attribute (e.g. "long_call", "short_call", "long_put", "short_put")

### D-03: Covered calls excluded from spread analytics
- Covered calls (stock buy + option write) are NOT spreads — they're sequential trades placed at different times
- A covered call "campaign" (stock position + multiple option writes over its lifetime) is a future-phase concept
- Phase 16 only handles true multi-leg spreads placed simultaneously via PlaceMultiLegOrder
- **Claude's Discretion:** Whether to add a `campaign_key` attribute for future covered call linking, or defer entirely

### D-04: Spread P&L calculation
- **Claude's Discretion** — recommended approach: sum per-leg realized P&L from v_order_pnl, GROUP BY spread_group_key
- This leverages existing views and is simple. A spread is profitable if net P&L > 0
- Handle partial fills: if any leg is unfilled, the spread group has status "partial"

### D-05: SPREAD-02 scope — all options strategies
- Spread_group_key support at the Go server level (centralized per D-02)
- Any strategy using PlaceMultiLegOrder gets automatic spread grouping — no per-strategy code needed
- Python strategies don't need changes — the Go server handles it

### D-06: Dashboard design (DASH-05) — Metrics + timeline
- Spread P&L summary: win/loss count, total net P&L, avg spread return
- Spread P&L over time (line chart)
- Per-strategy spread breakdown table
- Filter by playground + strategy
- Added to `infra/provision-metabase.py` following existing pattern

### D-07: Testing — seed script + unit tests + integration test
- Python seed script inserts mock spread orders via RPC for testing views
- Unit tests verify SQL view aggregation
- Integration test with TestContainers for full E2E (Go server + Postgres)
- Both approaches as requested

### Claude's Discretion
- Exact spread_group_key format (UUID vs descriptive)
- leg_role taxonomy (how many role types)
- Whether to add campaign_key for future covered call linking or defer
- SQL view design details for v_spread_pnl
- Dashboard layout and card arrangement for DASH-05
- Which integration test patterns to follow from existing TestContainers tests

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Go Server — Multi-Leg Orders
- `src/go/playground.proto` — `PlaceMultiLegOrder` RPC and `PlaceMultiLegOrderRequest` message (line ~18, ~314)
- `src/go/backtester-api/router/grpc.go` — PlaceMultiLegOrder handler implementation
- `src/go/backtester-api/models/order_record.go` — OrderRecord model with attributes JSONB

### SQL Views
- `infra/analytics-schema.sql` — Existing views pattern, extend with spread views
- `infra/provision-metabase.py` — Dashboard provisioning (extend with DASH-05)

### Python Strategies
- `src/clients/python/strategies/credit_spread.py` — CreditSpreadStrategy (primary multi-leg strategy)
- `src/clients/python/strategies/covered_call.py` — OptionsStrategyBasic (excluded from spread grouping)
- `src/clients/python/engine/client.py` — BacktesterPlaygroundClient with PlaceMultiLegOrder support

### Testing
- `integration_testing/` — Existing E2E test patterns
- `src/clients/python/tests/` — Existing Python test patterns

### Prior Phase Artifacts
- `.planning/phases/13-analytics-schema-indexes/13-01-SUMMARY.md` — SQL views pattern
- `.planning/phases/14-core-performance-dashboards/14-CONTEXT.md` — Dashboard-as-code decision

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `PlaceMultiLegOrder` RPC already exists — just needs to set spread_group_key in attributes
- `order_records.attributes` JSONB column already stores metadata (e.g. `ev`, `underlying_price`)
- `v_order_pnl` already computes per-order realized P&L — spread view can GROUP BY on it
- `provision-metabase.py` pattern for adding dashboards programmatically
- TestContainers setup in `integration_testing/`

### Established Patterns
- JSONB attributes on order_records for extensible metadata
- SQL views in `infra/analytics-schema.sql` with CREATE OR REPLACE
- Dashboard-as-code via Metabase REST API

### Integration Points
- Go server `PlaceMultiLegOrder` handler — add spread_group_key + leg_role to attributes
- SQL view joining v_order_pnl with attributes JSONB extraction
- provision-metabase.py — new dashboard section

</code_context>

<specifics>
## Specific Ideas

- Credit spread is the primary use case: buy one option + sell another at different strikes
- A spread group should show: strategy type, symbol, net credit/debit, net P&L, status (open/closed/partial)
- Spread P&L over time chart should show cumulative net P&L across all spread groups

</specifics>

<deferred>
## Deferred Ideas

- Covered call campaign linking (stock position + multiple option writes) — future phase
- Iron condor decomposition (two spreads as one trade unit) — future enhancement
- Spread risk metrics (max loss, breakeven prices) — future enhancement
- Greeks aggregation per spread — out of scope

</deferred>

---

*Phase: 16-spread-analytics*
*Context gathered: 2026-03-30*
