# Design — portfolio-risk-overlay

## Approach

A pre-trade risk gate that sits between strategy output (an order proposed via the Simulation `PlaceOrder` path) and submission to the Broker seam. The heart of the change is a **pure, side-effect-free evaluation engine** (`Evaluate`) that takes an immutable snapshot of portfolio state plus one proposed order and returns an allow/reject `Decision` with a typed list of breaches. Keeping the engine pure is what makes every limit-breach scenario deterministically unit-testable without a database, broker, or clock — the core requirement of the overnight Tier B scope.

Two capabilities split the work along a clean seam:

- **`portfolio-risk-limits`** — the deterministic limit math (gross/net exposure, sector concentration, 5-session drawdown breaker, per-strategy EV-weighted allocation caps, crowding consumption, and the entry-vs-reduction classification). No I/O.
- **`simulation-risk-gate`** — the impure adhesive: YAML config loading, the `CrowdingLookup` seam that reads the crowding tables, and the single wiring point on the Simulation order path, guarded so Paper/Margin are never touched.

## File / package layout

New package `src/go/tradingstack/riskoverlay/` (sibling to `tradingstack/crowding/`):

- `state.go` — `PortfolioState` (logical positions with signed notional + sector, per-strategy deployed capital, trailing equity series), `ProposedOrder` (ticker, sector, strategy ID, signed notional, and whether it opens/increases vs. reduces a logical position), `RiskLimits`.
- `engine.go` — `Evaluate(state, order, limits, crowding) (Decision, error)`; `Decision`, `LimitBreach`, `LimitType`.
- `config.go` — `LoadRiskLimits(...)` following the options-config YAML pattern; `DefaultRiskLimits`; validation sentinel errors.
- `crowding.go` — `CrowdingView`, the `CrowdingLookup` interface, a `GormCrowdingLookup` reading `crowding_metrics` / `crowding_flagged_candidates`, and a `FakeCrowdingLookup` for tests.
- `gate.go` — `SimulationRiskGate` adapter: builds `PortfolioState` from the live playground snapshot, resolves the `CrowdingView`, calls `Evaluate`, and returns allow/reject. Invoked only from the Simulation branch of the order path.
- `*_test.go` — fixture-driven unit tests, one per breach family plus boundary, reduction-order, crowding, and config cases.

Consumed read-only from other changes: `StrategyEvWeight` (`strategy_ev_weights`) from `tradingstack`, and `CrowdingMetric` / `CrowdingFlaggedCandidate` from `tradingstack/crowding`.

## Data flow

1. Simulation `PlaceOrder` builds a `ProposedOrder` from the incoming request and a `PortfolioState` snapshot from the playground's logical positions, per-strategy deployed capital, and trailing equity series.
2. The gate resolves the current scan cycle's `CrowdingView` via `CrowdingLookup` and loads `RiskLimits` from config (cached).
3. `Evaluate` runs the limit checks in a fixed order and accumulates breaches. Reduction orders short-circuit to `Allowed` before any limit is checked.
4. `Allowed` → placement proceeds unchanged. Rejected → the order is not committed to the order queue and the breach list is returned to the caller.

## Wiring point

The single integration seam is the Simulation branch of the order-placement path (`Server.PlaceOrder` → `Playground.PlaceOrder`, guarded on `PlaygroundEnvironmentSimulator`). Paper and Margin branches are left byte-for-byte unchanged. The gate is a no-op (allow-all) when its enablement flag is off, so Simulation output is identical to today when disabled.

## Explicitly OUT of scope

- **Any Paper or Margin enablement.** The gate is Simulation-only; extending it to real order paths is a separate, operator-gated change.
- **True statistical correlation clustering.** "Correlation concentration" is approximated by *sector* grouping in this change; a returns-covariance correlation model is deferred.
- **Order resizing / partial fills.** The gate rejects or allows whole orders; it never trims an order to fit under a limit.
- **Writing any new table.** This change reads `strategy_ev_weights` and the crowding tables; it creates and migrates nothing.
- **New Taskfile deploy/infra targets** and any live-data or broker validation.

## Dependency ordering on other batch changes

Must be authored and merged **after** both:

1. `trading-stack-schema` — provides `StrategyEvWeight` / `strategy_ev_weights` (per-strategy EV-weighted caps).
2. `crowding-detection` — provides `CrowdingMetric` / `CrowdingFlaggedCandidate` (the flagged-candidate read surface).

No other batch change depends on this one.

## Verification gates

- **Unit tests for every limit-breach scenario** — gross, net, sector concentration, drawdown breaker (trip + boundary), per-strategy allocation (including zero-weight strategy), crowding, and the reduction-order-always-passes invariant; plus config-loader default and validation cases. These are the primary overnight gate.
- **G1** — `go build ./src/go/... ./cmd/...` green.
- **G2** — `task test` green (existing `backtester-api` suite unaffected).
- **G6 — MANDATORY Fable adversarial review.** This is trade-gating safety-rail code; per the overnight plan it receives a G6 review unconditionally before commit. Reviewer must confirm the Simulation-only guard, the reduction-order bypass, and every boundary (`>` vs `>=`) comparison.

## Deferred validation (Tier B)

Per the overnight plan, only the Simulation-path wiring plus full unit-test coverage land tonight. Deferred to a later, operator-gated change: enabling the gate on Paper/Margin, integrating real portfolio state from live playgrounds end-to-end, and any live-feed or broker-sandbox validation. No prod DB, no live/paper broker orders, no new external infrastructure is touched by this change.
