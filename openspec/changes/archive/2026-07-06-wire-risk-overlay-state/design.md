# Design — wire-risk-overlay-state

## Context

`portfolio-risk-overlay` landed the pure engine (`Evaluate`), the Simulation adapter (`SimulationRiskGate`), the config loader, and the hook: `Playground.PlaceOrder` already calls `CheckRiskGate(p, order)` on the Simulation branch, composed AFTER the kill-switch `CheckOrderGate`. What is missing is everything impure: nothing calls `models.SetRiskGate`, the `PortfolioSnapshotFunc` is nil, and `riskOverlay.enabled` defaults to false. HANDOFF.md's review-nit inventory names five overlay nits (a–e) to fold into this wiring change. ADR-0005 removed OpenTelemetry; all new signals must use the internal telemetry registry (`src/go/telemetry`).

## Goals

- Real `PortfolioState` from a live Simulation playground: positions with sector and signed notional, per-strategy deployed capital, trailing 5-session equity series, EV weights.
- Gate installed at startup and enabled by default for Simulation, with shipped permissive default limits (enforcement begins when the operator narrows them).
- Every fail-permissive permit visible in telemetry (nit b); EV-family pin state visible and alert-worthy (nit a); option notional correct (nit c); wiped-out equity fails safe (nit d); honest wording (nit e).

## Non-Goals

- **No Paper or Margin gating.** The hook is structurally Simulation-only (`p.Meta.Mode == ModeSimulation`); extending it is a separate operator-gated change.
- No order resizing/trimming; the gate still rejects or allows whole orders.
- No new tables, no migrations, no proto changes, no Python client changes.
- No regime-aware EV-weight selection (latest row per strategy regardless of regime; refinement deferred).
- No true correlation clustering (sector proxy stands, per the parent change).

## Decisions

### D1 — Snapshot builder assembled from playground in-memory state, DB only for EV weights and sectors

`BuildPortfolioSnapshot` (the production `PortfolioSnapshotFunc`) reads:

- **Positions**: the playground's position cache (`GetPositionCache`) — signed notional per instrument as `quantity × current price` (falling back to cost basis when no current price), with the option ×100 multiplier applied for option instruments.
- **Equity series**: the playground's in-memory equity plot (`GetEquityPlot`), collapsed to one closing value per distinct session date, last 5 sessions, ending with current equity (`GetEquity`). No DB round-trip, so equity can never be the fail-permissive trigger.
- **Strategy deployed capital**: open positions' absolute notional attributed to a strategy ID derived from the opening order's `Tag` (empty tag → the playground's client ID as the fallback strategy identity). Same attribution derives `ProposedOrder.StrategyID` from the incoming order.
- **EV weights**: an `EvWeightLookup` seam (`Latest() (map[string]float64, error)`) with a GORM implementation reading the most recent `strategy_ev_weights` row per `strategy_id`, and an in-memory fake for tests. *Alternative rejected*: querying inside the engine — breaks engine purity, the parent change's core property.
- **Sector**: a `SectorLookup` seam (`SectorOf(ticker) (string, error)`) with a GORM implementation reading the latest non-null `scan_results.sector` for the ticker, plus an in-memory fake. Unknown ticker → empty sector: the engine already skips the sector-concentration check for an empty sector, and each unknown-sector resolution increments the degradation counter with `reason=sector_unknown` (the family is partially blind, and now visibly so). *Alternative rejected*: a static ticker→sector YAML map — a second config to drift; `scan_results` is already the platform's sector source of truth.

Lookups are consulted per evaluation with a short (scan-cycle-scale, ~60s) in-process cache to keep `PlaceOrder` latency flat; reduction orders never reach any lookup.

### D2 — Reduction bypass stays ahead of everything (unchanged, load-bearing)

The side-only reduction short-circuit in `EvaluateSimulationOrder` runs before snapshot build and all lookups, and the engine's `IsReduction` short-circuit stays first. The parent design's N1 dependency note still holds: this is safe only because `Playground.placeOrder` re-derives closeable volume and rejects oversized closes, so a side-classified reduction can only reduce. This change re-asserts that invariant in tests; it does not touch the netting layer.

### D3 — Fail-permissive stays, but becomes observed (nits b, e)

A snapshot-build or crowding-lookup error still permits the (non-reducing) order — halting on DB hiccups is the kill switch's job, not the overlay's. What changes: each such permit increments `grodt.riskoverlay.degraded` (Counter, label `reason` ∈ `snapshot_error` | `crowding_lookup_error` | `sector_unknown`) in `telemetry.Default`, in addition to the existing Warn log. The `AlertEngine` gains a rule alerting on sustained degradation (threshold over its evaluation window, mirroring the errors-total rule pattern), so a blind-but-permitting gate pages the operator without Grafana. *Alternative rejected*: fail-closed on DB errors — turns infra flakiness into trading halts and inverts the overlay's purpose. Wording fix (nit e): the **disabled** gate state is documented as **permissive-blind** (no evaluation, nothing recorded); the fail-permissive path is now honestly *permissive-observed*. The misleading "permissive-observe" comment in `gate.go` and the parent proposal language are corrected wherever they appear in living artifacts.

### D4 — Empty EV set: family pinned inactive, telemetered, alert-worthy (nit a)

The engine's existing behavior (empty `EvWeights` map ⇒ strategy-allocation family skipped) is kept — with no EV rows at all, capping every strategy at zero would halt all entries platform-wide, which is the drawdown breaker's job, not a data-availability artifact. The spec now states this pin explicitly, and the gate exposes it: gauge `grodt.riskoverlay.ev_family_active` (1/0) and gauge `grodt.riskoverlay.enabled` (1/0), with an `AlertEngine` rule raising an operator Alert when the gate is enabled while the EV family is pinned inactive (re-notify-until-acked per existing alert semantics). A **non-empty** set keeps the strict rule: unlisted strategy ⇒ zero cap ⇒ every entry rejected. *Alternative rejected*: treating empty-set as all-zero caps — see above.

### D5 — Non-positive equity peak trips the breaker (nit d)

`drawdownPct` currently returns not-ok for `peak <= 0`, silently disabling the breaker exactly when the book is annihilated. New behavior: a **non-empty** equity series whose 5-session peak is `<= 0` is treated as a tripped breaker — entries rejected with a `drawdown_breaker` breach reason naming the non-positive peak. An **empty** series stays breaker-inactive (no data is not catastrophe; and with D1 the wired snapshot always carries at least current equity, so empty only occurs in fixtures or pre-wiring). Reductions are untouched by construction. *Alternative rejected*: computing drawdown against `|peak|` — a negative-peak percentage is meaningless; a categorical trip is honest and safe.

### D6 — Option notional ×100 (nit c)

`MapProposedOrder` multiplies `|quantity| × price` by 100 when `order.Class == OrderRecordClassOption` (equity and unknown classes unchanged, since equity is the historical default for an empty class). The same multiplier is applied in the snapshot builder's position-notional math (D1) so both sides of the exposure comparison agree. Engine unchanged — notional correctness is a mapping concern.

### D7 — Default-enable via config default + startup installation

`DefaultRiskLimits.Enabled` flips to `true`; a commented sample `src/go/risk-overlay-config.yaml` ships at the already-resolved default path. `cmd/main.go` wires: resolve config → construct `GormEvWeightLookup`/`GormSectorLookup`/`GormCrowdingLookup` → `NewSimulationRiskGate(...)` with `BuildPortfolioSnapshot` → `models.SetRiskGate(gate)`, placed immediately after the kill-switch `SetOrderGate` block so the composition order is visible at the wiring site. An invalid config file fails startup loudly (existing `ErrInvalidRiskLimits` path); an absent file yields the permissive defaults. Because defaults are permissive (1e15 caps, 100% percentages, crowding off), default-enabled changes no order outcome until the operator narrows limits — but the gate is now live, telemetered, and honest. Operator disable remains one YAML line (`enabled: false`).

## Risks → Mitigations

- **Gate rejects orders existing Simulation flows expect to pass** → shipped defaults are permissive; `task test:smoke` runs in the verification gates; rejection surfaces as a typed `RejectedError` with the full breach list.
- **PlaceOrder latency from per-order DB lookups** → reductions short-circuit pre-I/O; EV/sector/crowding lookups cached at scan-cycle granularity; equity/positions are in-memory.
- **Alert nag from permanently empty `strategy_ev_weights` in dev** → the EV-pin alert is ackable and its rule threshold is config-tunable via the existing telemetry config surface; the condition is real (allocation family inactive) and should be visible.
- **Breaker fail-safe (D5) trips on garbage equity data** → only a non-positive *peak* trips it (a single bad tick below zero with a positive peak just reads as deep drawdown); unit fixtures pin both directions.
- **Safety regression at the Broker seam** → mandatory adversarial review gate (same as parent change) explicitly re-confirms: kill-switch-first composition, Simulation-only guard, reduction bypass ahead of all I/O, and every `>` vs `>=` boundary.

## Migration / Rollback

- **Migration**: none — no schema changes. Deploy is code-only; on first boot the gate installs and reports `grodt.riskoverlay.enabled = 1`.
- **Rollback (operator, no deploy)**: set `riskOverlay.enabled: false` in `risk-overlay-config.yaml` and restart — the gate becomes permissive-blind, Simulation behaves exactly as today.
- **Rollback (code)**: revert the wiring commit; `SetRiskGate` is never called, `CheckRiskGate` sees a nil gate and permits everything — the pre-change state by construction.
