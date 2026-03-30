# Phase 16: Spread Analytics - Research

**Researched:** 2026-03-30
**Domain:** Multi-leg option spread grouping, SQL analytics views, Metabase dashboards
**Confidence:** HIGH

## Summary

Phase 16 adds spread analytics by (1) injecting `spread_group_key` and `leg_role` attributes into order_records when `PlaceMultiLegOrder` is called, (2) creating a SQL view that aggregates per-leg P&L from `v_order_pnl` by spread group, and (3) provisioning a Metabase dashboard for spread performance.

The existing infrastructure supports this well. The `order_records.attributes` JSONB column already stores arbitrary metadata, `v_order_pnl` already computes per-order realized P&L, and `provision-metabase.py` has a well-established pattern for adding dashboards programmatically. The main code changes are in the Go `PlaceMultiLegOrder` handler (generate UUID key, inject into each leg's attributes) and a new SQL view in `analytics-schema.sql`.

**Primary recommendation:** Generate a UUID `spread_group_key` in the Go `PlaceMultiLegOrder` handler, inject it plus `leg_role` into each leg's `CreateOrderRequest.Attributes`, add `v_spread_pnl` SQL view grouping by `spread_group_key`, and add a 5th dashboard section to `provision-metabase.py`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Attributes JSONB column on order_records -- store spread_group_key and leg_role in existing attributes JSONB field. No new tables.
- D-02: Go server generates spread_group_key when processing PlaceMultiLegOrder RPC (centralized).
- D-03: Covered calls excluded from spread analytics. Only true multi-leg spreads via PlaceMultiLegOrder.
- D-04: Sum per-leg realized P&L from v_order_pnl, GROUP BY spread_group_key.
- D-05: All options strategies using PlaceMultiLegOrder get automatic grouping -- no per-strategy code needed.
- D-06: Dashboard design (DASH-05) -- metrics + timeline. Added to provision-metabase.py.
- D-07: Testing -- seed script + unit tests AND integration test with TestContainers.

### Claude's Discretion
- Exact spread_group_key format (UUID vs descriptive)
- leg_role taxonomy (how many role types)
- Whether to add campaign_key for future covered call linking or defer
- SQL view design details for v_spread_pnl
- Dashboard layout and card arrangement for DASH-05
- Which integration test patterns to follow from existing TestContainers tests

### Deferred Ideas (OUT OF SCOPE)
- Covered call campaign linking (stock position + multiple option writes) -- future phase
- Iron condor decomposition (two spreads as one trade unit) -- future enhancement
- Spread risk metrics (max loss, breakeven prices) -- future enhancement
- Greeks aggregation per spread -- out of scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SCHEMA-03 | spread_groups and spread_group_legs tables for multi-leg option grouping | D-01 overrides: no new tables, use attributes JSONB + SQL view instead |
| SCHEMA-04 | SQL view for spread-aware P&L (net spread profit, not per-leg) | v_spread_pnl view using GROUP BY on attributes->>'spread_group_key' over v_order_pnl |
| SPREAD-01 | Go server registers spread legs via spread_group_key attribute on PlaceMultiLegOrder | PlaceMultiLegOrder handler modification -- generate UUID, inject into CreateOrderRequest.Attributes |
| SPREAD-02 | Python strategies emit spread_group_key for multi-leg orders | Handled server-side per D-05 -- Python strategies need no changes |
| DASH-05 | Spread analytics dashboard: spread P&L, spread win/loss ratio | New section in provision-metabase.py following existing pattern |
</phase_requirements>

## Architecture Patterns

### Critical Code Gaps Found

**1. PlaceMultiLegOrder has no attributes support:**
The current `PlaceMultiLegOrderRequest` proto message (line 314) does NOT have an `attributes` field. The `MultiLegOrderLeg` message also lacks attributes. The handler (grpc.go:1130) builds `CreateOrderRequest` structs without setting `Attributes`. This means the Go handler must generate AND inject attributes entirely server-side -- it cannot accept them from the client.

**2. Credit spread strategy uses individual place_order calls:**
`CreditSpreadStrategy` (credit_spread.py:935-958) places legs as two separate `place_order` calls, NOT via `PlaceMultiLegOrder`. It already stores a `group_id` in attributes. Per D-05, the server-side approach only applies to `PlaceMultiLegOrder` -- the credit spread strategy would need to be migrated to use `PlaceMultiLegOrder` to benefit from automatic spread grouping. However, per D-03, credit spread's existing `group_id` attribute already provides grouping.

**Recommendation:** Since the credit spread strategy already has its own `group_id` in attributes, and SPREAD-02 says "all options strategies using PlaceMultiLegOrder get automatic grouping," the v_spread_pnl view should recognize BOTH sources: server-injected `spread_group_key` AND existing `group_id` from the credit spread strategy. This makes the view immediately useful for existing data.

### Recommended Changes

```
src/go/
  playground.proto                    # No change needed -- attributes generated server-side
  backtester-api/router/grpc.go       # PlaceMultiLegOrder: generate UUID, inject into each leg
infra/
  analytics-schema.sql                # Add v_spread_pnl view
  provision-metabase.py               # Add Dashboard 5: Spread Analytics
integration_testing/
  spread_analytics_e2e_test.go        # New: E2E test for spread grouping + view
```

### Pattern: Server-Side Attribute Injection

The Go `PlaceMultiLegOrder` handler should:
1. Generate a UUID spread_group_key
2. For each leg in the request, set `Attributes["spread_group_key"] = key`
3. Determine `leg_role` from the leg's side: sell_to_open -> "short", buy_to_open -> "long"
4. Set `Attributes["leg_role"] = role`

```go
// In PlaceMultiLegOrder handler, before building CreateOrderRequest slice:
spreadGroupKey := uuid.New().String()

for _, leg := range req.Legs {
    legRole := "unknown"
    side := models.TradierOrderSide(leg.Side)
    if side == models.TradierOrderSideSellToOpen || side == models.TradierOrderSideSellShort {
        legRole = "short"
    } else if side == models.TradierOrderSideBuyToOpen || side == models.TradierOrderSideBuy {
        legRole = "long"
    }

    attrs := map[string]string{
        "spread_group_key": spreadGroupKey,
        "leg_role":         legRole,
    }

    requests = append(requests, &models.CreateOrderRequest{
        // ... existing fields ...
        Attributes: attrs,
    })
}
```

### Pattern: SQL View for Spread P&L

```sql
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
    MIN(o.timestamp) AS entry_time,
    MAX(o.timestamp) AS last_leg_time,
    ARRAY_AGG(DISTINCT o.symbol) AS symbols,
    ARRAY_AGG(DISTINCT o.attributes->>'leg_role') AS roles
FROM order_records o
JOIN v_order_pnl pnl ON pnl.order_id = o.id
WHERE o.deleted_at IS NULL
  AND (o.attributes->>'spread_group_key' IS NOT NULL
       OR o.attributes->>'group_id' IS NOT NULL)
  AND o.side IN ('buy_to_open', 'sell_to_open', 'buy', 'sell_short')
GROUP BY
    COALESCE(o.attributes->>'spread_group_key', o.attributes->>'group_id'),
    o.playground_id;
```

### Discretion Recommendations

**spread_group_key format: UUID.** Consistent with playground IDs, no collision risk, already imported in grpc.go. Descriptive keys add complexity for no analytical benefit.

**leg_role taxonomy: Two roles.** `short` and `long` -- derived from order side. Simple, covers all current spread types (credit spreads, debit spreads). More specific roles (e.g., "short_call", "long_put") can be derived by joining with the order's symbol/class at query time.

**campaign_key: Defer.** The credit spread strategy already has `group_id` in attributes. Adding a separate `campaign_key` concept adds complexity for a deferred feature. When covered call campaigns are implemented, they can use their own attribute key.

**GIN Index:** Add a GIN index on `order_records.attributes` for JSONB key extraction performance:
```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_order_records_attributes_gin
  ON order_records USING gin (attributes jsonb_path_ops);
```

### Anti-Patterns to Avoid
- **Double-counting P&L:** The view must only use opening-side orders (buy_to_open, sell_to_open) to match how v_playground_stats avoids double-counting. Closing-side orders' P&L overlaps with opening-side CalcRealizedPL.
- **Filtering only on spread_group_key:** Must also recognize `group_id` from credit_spread.py to capture existing data.
- **Adding proto attributes field:** Per D-02, the server generates the key -- no need to modify the proto. Adding client-side attributes to PlaceMultiLegOrder would contradict centralized generation.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Spread P&L calculation | Custom Go aggregation code | SQL view over v_order_pnl | Leverages existing verified P&L logic, no new code to maintain |
| UUID generation | Custom key format | `uuid.New().String()` | Already imported, collision-free |
| Dashboard provisioning | Manual Metabase setup | Extend provision-metabase.py | Idempotent, version-controlled, follows existing pattern |
| JSONB querying | Custom Go-side spread lookups | PostgreSQL `attributes->>'key'` | Native JSONB support, indexable with GIN |

## Common Pitfalls

### Pitfall 1: P&L Double-Counting
**What goes wrong:** Including both opening and closing side orders in the spread P&L sum, resulting in inflated numbers.
**Why it happens:** v_order_pnl has rows for all filled orders. Opening orders show P&L from ClosedBy trades, closing orders show P&L from Closes -- they overlap.
**How to avoid:** Filter v_spread_pnl to only `side IN ('buy_to_open', 'sell_to_open', 'buy', 'sell_short')` -- same filter as v_playground_stats.
**Warning signs:** Spread net P&L is exactly double what expected.

### Pitfall 2: Partial Spread Status
**What goes wrong:** Treating a spread as "closed" when only one leg is filled.
**Why it happens:** In backtesting, fills are nearly instant. In live mode, one leg might get rejected.
**How to avoid:** Include `BOOL_AND(o.status = 'filled') AS all_filled` and `COUNT(*) AS leg_count` in the view. Dashboard queries should flag spreads where `leg_count < 2` or `NOT all_filled`.
**Warning signs:** Spreads with only 1 leg showing unrealistic P&L.

### Pitfall 3: Null Attributes
**What goes wrong:** Orders placed before this feature have NULL attributes, causing the GIN index to not cover them.
**Why it happens:** Existing orders don't have spread_group_key.
**How to avoid:** The WHERE clause `attributes->>'spread_group_key' IS NOT NULL` naturally excludes pre-existing orders. No migration needed.

### Pitfall 4: Option Multiplier
**What goes wrong:** Forgetting the 100x multiplier for option P&L.
**Why it happens:** v_order_pnl already handles the multiplier (`WHEN class = 'option' THEN ... * 100`). But if building any custom aggregation, this could be missed.
**How to avoid:** Always use v_order_pnl as the P&L source -- it handles multipliers correctly.

## Code Examples

### PlaceMultiLegOrder Handler Modification
```go
// Source: grpc.go:1104 -- modify existing handler
func (s *Server) PlaceMultiLegOrder(ctx context.Context, req *pb.PlaceMultiLegOrderRequest) (*pb.PlaceMultiLegOrderResponse, error) {
    // ... existing checkOrderExists ...
    // ... existing playgroundID parse ...

    // Generate spread group key for all legs
    spreadGroupKey := uuid.New().String()

    var requests []*models.CreateOrderRequest
    for _, leg := range req.Legs {
        // Determine leg role from side
        legRole := "unknown"
        switch models.TradierOrderSide(leg.Side) {
        case models.TradierOrderSideSellToOpen, models.TradierOrderSideSellShort:
            legRole = "short"
        case models.TradierOrderSideBuyToOpen, models.TradierOrderSideBuy:
            legRole = "long"
        }

        attrs := map[string]string{
            "spread_group_key": spreadGroupKey,
            "leg_role":         legRole,
        }

        var closeOrderId *uint
        if leg.CloseOrderId != nil {
            closeOrderId = new(uint)
            *closeOrderId = uint(*leg.CloseOrderId)
        }

        requests = append(requests, &models.CreateOrderRequest{
            Symbol:          leg.Symbol,
            ClientRequestID: req.ClientRequestId,
            Class:           models.OrderRecordClass(leg.AssetClass),
            Quantity:        leg.Quantity,
            Side:            models.TradierOrderSide(leg.Side),
            OrderType:       models.OrderRecordType(req.Type),
            Duration:        models.OrderRecordDuration(req.Duration),
            Tag:             leg.Tag,
            CloseOrderId:    closeOrderId,
            IsAdjustment:    false,
            Attributes:      attrs,
        })
    }

    orders, err := s.dbService.PlaceOrders(playgroundID, requests)
    // ... rest unchanged ...
}
```

### SQL View: v_spread_pnl
```sql
-- Source: Pattern follows analytics-schema.sql conventions
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
```

### Spread Stats Aggregation View
```sql
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
```

### Dashboard Card Examples
```python
# Source: Pattern follows provision-metabase.py conventions
def build_spread_analytics_cards(session, base_url, db_id):
    cards = {}

    cards["spread_summary"] = upsert_card(session, base_url, _make_card(
        "Spread: Summary Stats", db_id,
        """SELECT total_spreads, total_net_pnl,
               ROUND(win_rate * 100, 1) AS win_rate_pct,
               avg_spread_pnl, avg_win, avg_loss
        FROM v_spread_stats
        WHERE playground_id = {{playground_id}}::uuid""",
        display="table",
    ))

    cards["spread_pnl_timeline"] = upsert_card(session, base_url, _make_card(
        "Spread: P&L Over Time", db_id,
        """SELECT entry_time, net_pnl,
               SUM(net_pnl) OVER (ORDER BY entry_time) AS cumulative_spread_pnl
        FROM v_spread_pnl
        WHERE playground_id = {{playground_id}}::uuid
          AND all_filled
        ORDER BY entry_time""",
        display="line",
        viz_settings={
            "graph.x_axis.column": "entry_time",
            "graph.metrics": ["cumulative_spread_pnl"],
        },
    ))

    # ... more cards for win/loss ratio, per-spread detail table ...
    return cards
```

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.9.0, TestContainers v0.35.0 |
| Config file | taskfile.yml (test tasks) |
| Quick run command | `cd src/go/backtester-api && go test -count=1 -run TestSpread ./...` |
| Full suite command | `task test` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SPREAD-01 | PlaceMultiLegOrder injects spread_group_key + leg_role into attributes | unit | `go test -count=1 -run TestPlaceMultiLegOrderAttributes ./src/go/backtester-api/...` | No -- Wave 0 |
| SCHEMA-03 | Attributes JSONB stores spread grouping (no new tables) | integration | TestContainers E2E | No -- Wave 0 |
| SCHEMA-04 | v_spread_pnl view aggregates spread P&L correctly | integration | `psql -f infra/analytics-schema.sql` + SQL assertions | No -- Wave 0 |
| SPREAD-02 | Strategies using PlaceMultiLegOrder get auto-grouping | integration | E2E: place multi-leg, query view | No -- Wave 0 |
| DASH-05 | Spread dashboard provisioned with correct cards | manual | `python infra/provision-metabase.py` + visual check | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `cd src/go/backtester-api && go test -count=1 -run TestSpread ./...`
- **Per wave merge:** `task test`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] Unit test for PlaceMultiLegOrder attribute injection (Go)
- [ ] SQL view verification test (seed data + assert spread P&L)
- [ ] Integration test extending `tradier_place_spread_test.go` pattern

## Sources

### Primary (HIGH confidence)
- `src/go/backtester-api/router/grpc.go:1104-1160` -- PlaceMultiLegOrder handler (verified current code)
- `src/go/backtester-api/models/order_record.go:49-81` -- OrderRecord with Attributes JSONB (verified)
- `src/go/backtester-api/models/create_order_request.go:22` -- CreateOrderRequest.Attributes field (verified)
- `infra/analytics-schema.sql` -- Existing views pattern, v_order_pnl logic (verified)
- `infra/provision-metabase.py` -- Dashboard provisioning pattern (verified)
- `src/clients/python/strategies/credit_spread.py:889-958` -- Credit spread already uses group_id in attributes (verified)
- `src/go/playground.proto:305-323` -- MultiLegOrderLeg and PlaceMultiLegOrderRequest (verified: NO attributes field)
- `integration_testing/tradier_place_spread_test.go` -- Existing spread E2E test pattern (verified)

### Secondary (MEDIUM confidence)
- PostgreSQL JSONB GIN index performance for key extraction queries -- standard PostgreSQL pattern

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all tools are existing project code (Go, PostgreSQL, Metabase, existing views)
- Architecture: HIGH -- extends well-established patterns (attributes JSONB, SQL views, provision script)
- Pitfalls: HIGH -- verified by reading CalcRealizedPL and v_order_pnl source code

**Key discovery:** The credit spread strategy places legs via individual `place_order` calls with a `group_id` attribute, NOT via `PlaceMultiLegOrder`. The v_spread_pnl view should recognize both `spread_group_key` (from PlaceMultiLegOrder) and `group_id` (from credit_spread.py) to capture all spread data.

**Key gap:** `PlaceMultiLegOrderRequest` proto has no `attributes` field. Per D-02, the server generates attributes, so no proto change is needed -- but this means strategies cannot override or add custom attributes on multi-leg orders.

**Research date:** 2026-03-30
**Valid until:** 2026-04-30 (stable -- extends existing patterns with no external dependency changes)
