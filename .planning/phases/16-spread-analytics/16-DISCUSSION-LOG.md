# Phase 16: Spread Analytics - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.

**Date:** 2026-03-30
**Phase:** 16-spread-analytics
**Areas discussed:** Spread grouping model, Key generation, Covered call treatment, P&L calculation, SPREAD-02 scope, Dashboard design, Testing

---

## Spread Grouping Model

| Option | Description | Selected |
|--------|-------------|----------|
| Attributes JSONB | Store in existing order_records.attributes | ✓ |
| New tables | spread_groups + spread_group_legs | |
| You decide | | |

**User's choice:** Attributes JSONB (Recommended)

---

## Spread Group Key Generation

| Option | Description | Selected |
|--------|-------------|----------|
| Python strategy | Strategy generates key, passes as attribute | |
| Go server | Auto-generates on PlaceMultiLegOrder | ✓ |
| You decide | | |

**User's choice:** Go server (centralized). User emphasized: "I would like the logic to be centralized on the go server."

---

## Covered Call as Spread

| Option | Description | Selected |
|--------|-------------|----------|
| Exclude from spread analytics | Focus on true multi-leg spreads only | ✓ |
| Link via campaign key | Separate campaign_key for stock+options linking | |
| You decide | Claude designs | |

**User's choice:** Custom — "A covered call should not be recorded as a spread; however, the initial order should be linked. Later on, once the call option expires, it is possible to write another covered call against the same pre-existing stock." Decision: Exclude from Phase 16, covered call campaign linking deferred.

---

## Spread P&L Calculation

| Option | Description | Selected |
|--------|-------------|----------|
| Sum per-leg realized P&L | Use v_order_pnl, GROUP BY spread_group_key | |
| Net premium calculation | Premium received - premium paid | |
| You decide | Claude picks | ✓ |

**User's choice:** You decide

---

## SPREAD-02 Scope

| Option | Description | Selected |
|--------|-------------|----------|
| CreditSpreadStrategy only | Narrow scope | |
| All options strategies | Future-proof | ✓ |
| You decide | | |

**User's choice:** All options strategies. "This should be implemented at the python base layer or on the golang server."

---

## Dashboard Design (DASH-05)

| Option | Description | Selected |
|--------|-------------|----------|
| Metrics + timeline | Summary + P&L over time + per-strategy table | ✓ |
| Detailed per-spread view | Leg-by-leg breakdown | |
| You decide | | |

**User's choice:** Metrics + timeline (Recommended)

---

## Testing Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Seed script + unit tests | Mock data via RPC, verify SQL views | ✓ |
| Integration test with TestContainers | Full E2E | ✓ |
| You decide | | |

**User's choice:** Both

---

## Claude's Discretion

- spread_group_key format, leg_role taxonomy
- Whether to add campaign_key for future covered call linking
- Spread P&L calculation approach (recommended: sum per-leg)
- SQL view design, dashboard layout

## Deferred Ideas

- Covered call campaign linking — future phase
- Iron condor decomposition
- Spread risk metrics, Greeks aggregation
