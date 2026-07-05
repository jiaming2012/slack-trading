# Wire Risk Overlay State

## Why

The portfolio risk overlay (`portfolio-risk-overlay`, archived 2026-07-05) delivered a fully tested pure limit engine and Simulation gate adapter — but deliberately **unwired and default-disabled**, because enabling it against a nil portfolio snapshot would be safety theater. Until the gate sees real positions, equity, and EV weights, the platform's five portfolio limit families protect nothing. This change supplies the real portfolio-state feed, installs the gate at the Broker seam (composed after the kill switch), and default-enables it in Simulation mode, while folding in the five review nits recorded in HANDOFF.md's review-nit inventory.

## What Changes

- **Portfolio state feed**: implement a production `PortfolioSnapshotFunc` that maps a live playground into the engine's `PortfolioState` — logical positions (ticker, sector, signed notional), per-strategy deployed capital, and the trailing 5-session equity series — plus lookup seams (GORM-backed + in-memory fakes) for EV weights (`strategy_ev_weights`) and sector attribution (latest `scan_results.sector` per ticker).
- **Gate installation**: call `models.SetRiskGate` during server startup, so the already-landed hook in `Playground.PlaceOrder` (composed AFTER the kill-switch `CheckOrderGate`, Simulation branch only) becomes live. The kill switch remains a separate, upstream gate; the overlay can never mask or disable it.
- **Default-enable in Simulation only**: `riskOverlay.enabled` defaults to true, and a sample `risk-overlay-config.yaml` ships with documented permissive defaults. The gate is structurally never consulted on Paper or Margin paths; enabling any future Paper/Margin gating stays explicitly operator-only and out of scope.
- **Review nit (a) — EV pin semantics**: an empty EV-weight set pins the strategy-allocation family **inactive**; this is now explicit in the spec, exposed as a telemetry gauge, and raises an operator Alert when the gate is enabled while the family is pinned inactive.
- **Review nit (b) — degradation counter**: every fail-permissive permit (snapshot build error, crowding-lookup error, unknown sector) increments a `grodt.riskoverlay.degraded` counter (labeled by reason) in the internal telemetry registry (`src/go/telemetry`), so Grafana-less alerting can see gate blindness. Warn logs alone no longer carry this signal.
- **Review nit (c) — option contract multiplier**: `MapProposedOrder` applies the ×100 contract multiplier when computing notional for option-class orders.
- **Review nit (d) — non-positive equity**: an equity series whose peak is non-positive now **trips** the drawdown breaker (fail-safe: a wiped-out book halts entries) instead of silently disabling it. An empty series (no data) remains breaker-inactive but counts as degradation.
- **Review nit (e) — wording**: "permissive-observe" is corrected to **permissive-blind** for the disabled-gate state everywhere it appears (comments, docs, spec text); the fail-permissive path is now genuinely *observed* via the degradation counter.
- Rejections are counted per limit family in the telemetry registry (`grodt.riskoverlay.rejections`), so an operator can always tell whether the overlay is live and what it is blocking.
- The **reduction-orders-bypass-everything** invariant is preserved intact: risk-reducing orders short-circuit before any snapshot build, lookup, or limit check.
- Uses ONLY the internal telemetry registry (`telemetry.Default`, `Counter`/`Gauge`, `AlertEngine`) — no OpenTelemetry (removed per ADR-0005). Not BREAKING: with the shipped permissive default limits the enabled gate blocks nothing until an operator narrows them.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `portfolio-risk-limits`: the strategy-allocation requirement makes the empty-EV-set pin semantics explicit (family inactive, signalled); the drawdown-breaker requirement gains fail-safe behavior for a non-positive equity peak.
- `simulation-risk-gate`: the deferred enablement requirement is replaced by installed-at-startup + default-enabled-for-Simulation; new requirements cover the real portfolio-state feed, the option ×100 notional mapping, gate telemetry (degradation + rejection counters, enablement/EV-pin gauges), and the EV-pin operator alert.

## Impact

- `src/go/tradingstack/riskoverlay/` — new snapshot builder + EV-weight/sector lookup seams and fakes; `MapProposedOrder` ×100 fix; engine drawdown fail-safe; telemetry recording; `DefaultRiskLimits.Enabled` flips to true; comment/wording fixes.
- `cmd/main.go` — startup wiring: construct lookups, build the gate from resolved config, `models.SetRiskGate(...)` (adjacent to the existing `SetOrderGate` kill-switch wiring).
- `src/go/telemetry/` — new riskoverlay counter/gauge instruments registered in `Init()`; one new `AlertEngine` rule (EV family pinned inactive while gate enabled).
- `src/go/options-config.yaml` sibling: new `src/go/risk-overlay-config.yaml` sample (already the resolved default path).
- Reads `strategy_ev_weights` and `scan_results` (read-only); no schema, migration, proto, or Python client changes; Paper/Margin order paths byte-for-byte unchanged.
- Depends on archived changes `portfolio-risk-overlay` (engine/adapter/hook) and `replace-otel-with-internal-telemetry` (registry + AlertEngine).
- Safety-critical trade-gating code: carries a mandatory adversarial review gate before commit, mirroring the parent change.
- Verification is Simulation/local-only: unit tests, `task test:portfolio-risk-overlay`, `go build`, `task test`, `task test:smoke`. No live/paper orders, no prod DB, no new infrastructure.
