# Tasks — portfolio-risk-overlay

## 1. Package scaffolding and state types

- [x] 1.1 Create `src/go/tradingstack/riskoverlay/` package skeleton (package `riskoverlay`), importing `github.com/jiaming2012/slack-trading/src/go/tradingstack` for `StrategyEvWeight` and `.../tradingstack/crowding` for the crowding models (read-only).
- [x] 1.2 Add `state.go` with `PortfolioState` (logical positions carrying ticker, sector, signed notional; per-strategy deployed capital; trailing equity series), `ProposedOrder` (ticker, sector, strategy ID, signed notional, `IsReduction bool`), and `RiskLimits` (`MaxGrossExposure`, `MaxNetExposure`, `MaxSectorConcentrationPct`, `MaxDrawdownPct`, `DeployableCapital`, `RejectCrowdedEntries`).
- [x] 1.3 Add `Decision`, `LimitBreach`, and the `LimitType` enum (`gross_exposure`, `net_exposure`, `sector_concentration`, `drawdown_breaker`, `strategy_allocation`, `crowding`) to `engine.go`.

## 2. Limit-evaluation engine (pure)

- [x] 2.1 Implement the entry-vs-reduction short-circuit in `Evaluate`: a `ProposedOrder` with `IsReduction` true returns `Decision{Allowed:true}` before any limit is checked.
- [x] 2.2 Implement gross and net exposure checks (strict `>` against `MaxGrossExposure` / `abs(net)` against `MaxNetExposure`).
- [x] 2.3 Implement sector concentration check (resulting single-sector share of gross exposure strictly `>` `MaxSectorConcentrationPct`).
- [x] 2.4 Implement the 5-session drawdown breaker: compute `(peak-current)/peak*100` over the trailing equity series; when strictly `>` `MaxDrawdownPct`, reject entries with a `drawdown_breaker` breach.
- [x] 2.5 Implement per-strategy EV-weighted allocation caps: `cap = DeployableCapital * ev_weight(strategy)/sum(ev_weight)`; absent strategy → weight 0 → cap 0; reject entry whose resulting deployed capital strictly `>` cap.
- [x] 2.6 Implement crowding consumption: when `RejectCrowdedEntries` and the order's ticker is in the `CrowdingView` flagged set, add a `crowding` breach.
- [x] 2.7 Accumulate all breaches (do not early-return on first breach) and set `Allowed = len(Breaches)==0`.

## 3. Config loader (options-config YAML pattern)

- [x] 3.1 Add `config.go` with `DefaultRiskLimits` (documented default values) and `LoadRiskLimits(...)` reading a `riskOverlay` YAML block via the existing options-config conventions.
- [x] 3.2 Add validation returning a sentinel error for negative numeric limits or percentage limits outside `0–100`; missing block/fields fall back to defaults without error.

## 4. Crowding lookup seam

- [x] 4.1 Add `crowding.go` with `CrowdingView` (flagged bool + ticker set) and the `CrowdingLookup` interface (`ViewForScanCycle(scannedAt) (CrowdingView, error)`).
- [x] 4.2 Implement `GormCrowdingLookup` reading `crowding_metrics` / `crowding_flagged_candidates` for a `scanned_at`.
- [x] 4.3 Implement `FakeCrowdingLookup` (in-memory, seedable) for unit tests.

## 5. Simulation-path gate adapter

- [x] 5.1 Add `gate.go` with `SimulationRiskGate` that builds `PortfolioState` from a playground snapshot, resolves the `CrowdingView`, loads `RiskLimits`, and calls `Evaluate`.
- [x] 5.2 Wire the gate into the Simulation branch of the order-placement path only (guarded on `PlaygroundEnvironmentSimulator`); on a rejecting `Decision`, do not commit the order to the queue and return the breach list to the caller.
- [x] 5.3 Add the enablement flag to the `riskOverlay` config (default **disabled**), loaded + validated + tested. INSTALLATION (`SetRiskGate` in server startup) and default-enablement for Simulation are DEFERRED to the follow-up change `wire-risk-overlay-state` — the same follow-up that supplies the real portfolio-state mapping — because enablement without real portfolio state evaluates a nil snapshot and is safety theater. _(Amended 2026-07-05, review-driven: original claimed "active for Simulation by default"; nothing installed the gate and the config had no `enabled` field.)_

## 6. Fixture-based unit tests (one per breach scenario)

- [x] 6.1 Clean-order test: all limits satisfied → `Allowed` true, no breaches.
- [x] 6.2 Gross exposure: over-cap rejected; exactly-at-cap allowed.
- [x] 6.3 Net exposure: over-cap rejected while gross within cap.
- [x] 6.4 Sector concentration: over-limit rejected (names the sector); at/under limit allowed.
- [x] 6.5 Drawdown breaker: 8%-drawdown entry rejected with `drawdown_breaker`; exactly-at-limit not tripped.
- [x] 6.6 Reduction-order-always-passes: closing order allowed even while the breaker is tripped.
- [x] 6.7 Per-strategy allocation: higher-EV strategy passes while lower-EV strategy is capped; strategy absent from EV set → rejected.
- [x] 6.8 Crowding: entry into flagged ticker rejected; non-flagged ticker allowed; `RejectCrowdedEntries=false` disables rejection.
- [x] 6.9 Multi-breach test: order breaching gross exposure and strategy allocation reports both breaches.
- [x] 6.10 Config-loader tests: missing block → defaults; negative/out-of-range limit → sentinel error.
- [x] 6.11 `FakeCrowdingLookup` test: seeded flagged cycle returns the expected flagged ticker set.

## 7. Taskfile wiring

- [x] 7.1 Add `test:portfolio-risk-overlay` target to `taskfile.yml` running `go test -count=1 ./src/go/tradingstack/riskoverlay/...` with `TRADING_PROJECT_DIR` set, matching the style of `test` and `test:crowding-detection`.

## 8. Verification and closeout

- [x] 8.1 Unit tests for every limit-breach scenario pass (`task test:portfolio-risk-overlay`) — gross, net, sector, drawdown (trip + boundary), strategy allocation (incl. zero-weight), crowding, reduction-order bypass, multi-breach, and config-loader cases.
- [x] 8.2 G1: `go build ./src/go/... ./cmd/...` — green.
- [x] 8.3 G2: `task test` — green (existing `backtester-api` suite unaffected).
- [x] 8.4 G6 (MANDATORY, safety-critical): Fable adversarial review approves the diff, explicitly confirming the Simulation-only guard, the reduction-order bypass, and every `>` vs `>=` boundary before commit.
- [x] 8.5 Run `openspec validate portfolio-risk-overlay --strict` — passes.
- [ ] 8.6 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time).
