# Tasks — wire-risk-overlay-state

## 1. Engine fixes (pure, no I/O)

- [x] 1.1 `engine.go` — drawdown fail-safe (nit d): a non-empty equity series with 5-session peak `<= 0` trips the breaker (reject entries with a `drawdown_breaker` breach naming the non-positive peak); an empty series stays breaker-inactive. Reductions untouched (short-circuit precedes the breaker).
- [x] 1.2 `engine.go` / `state.go` — document the empty-EV-set pin semantics (nit a) at the `EvWeights` field and the allocation check: empty set = family pinned inactive, non-empty set = unlisted strategy capped at zero. Behavior unchanged; wording made explicit.
- [x] 1.3 `engine_test.go` — new fixtures: all-non-positive equity trips the breaker; negative-values-with-positive-peak reads as ordinary deep drawdown (no categorical trip); empty series produces no `drawdown_breaker` breach; empty EV set produces no `strategy_allocation` breach.

## 2. Notional mapping (nit c)

- [x] 2.1 `gate.go` — `MapProposedOrder` applies ×100 to `|quantity| × price` when `order.Class == OrderRecordClassOption`; equity and empty class unchanged.
- [x] 2.2 `gate_test.go` — option order 3 × 2.50 maps to 750.00 notional; equity order unchanged; option reduction still short-circuits.

## 3. Lookup seams (EV weights, sector)

- [x] 3.1 `evweights.go` — `EvWeightLookup` interface (`Latest() (map[string]float64, error)`), `GormEvWeightLookup` reading the most recent `strategy_ev_weights` row per `strategy_id`, `FakeEvWeightLookup` (seedable), plus a short scan-cycle-scale cache.
- [x] 3.2 `sector.go` — `SectorLookup` interface (`SectorOf(ticker) (string, error)`), `GormSectorLookup` reading the latest non-null `scan_results.sector` per ticker, `FakeSectorLookup`, same cache treatment. Unknown ticker → empty sector (recorded as degradation in task 5.2).
- [x] 3.3 Seam unit tests with the fakes; GORM implementations covered by a testcontainers round-trip (mirroring the crowding lookup's pattern).

## 4. Portfolio snapshot builder

- [x] 4.1 `snapshot.go` — `BuildPortfolioSnapshot` (a `PortfolioSnapshotFunc` factory taking the lookups): positions from the playground position cache with signed notional (quantity × current price, cost-basis fallback, ×100 for option instruments) and sector via `SectorLookup`; per-strategy deployed capital attributed by opening-order tag (client-ID fallback); trailing 5-session equity series from the in-memory equity plot (one closing value per session date, ending with current equity — no DB read); EV weights via `EvWeightLookup`; `ProposedOrder` completed with sector + strategy ID.
- [x] 4.2 `snapshot_test.go` — seeded playground fixture yields expected `PortfolioState` (signed notionals, sectors, deployed capital, EV map, exactly the last 5 sessions ending at current equity); unknown-sector order evaluated with empty sector; end-to-end gate decision with fakes and no database.
- [x] 4.3 Re-assert the reduction-bypass-before-I/O invariant: with every lookup and the snapshot builder erroring, a sell-to-close order is permitted with no lookup consulted and no degradation recorded (design D2 / N1 dependency).

## 5. Telemetry and alerting (internal registry ONLY — no OTel, per ADR-0005)

- [ ] 5.1 `src/go/telemetry/metrics.go` — register instruments in `Init()`: `grodt.riskoverlay.degraded` (Counter), `grodt.riskoverlay.rejections` (Counter), `grodt.riskoverlay.enabled` (Gauge), `grodt.riskoverlay.ev_family_active` (Gauge). Nil-safe like existing instruments.
- [ ] 5.2 `gate.go` — record: degraded with `reason` label (`snapshot_error` / `crowding_lookup_error` / `sector_unknown`) on each fail-permissive permit alongside the existing Warn; rejections per breached family with `limit_type` label; ev_family_active from each EV lookup result. Disabled gate records nothing (permissive-blind).
- [ ] 5.3 `src/go/telemetry/alerts.go` — two `AlertEngine` rules with config-tunable thresholds: sustained riskoverlay degradation in the evaluation window; gate enabled while EV family pinned inactive. Both follow existing Slack delivery + re-notify-until-ack semantics.
- [ ] 5.4 Telemetry tests: degraded/rejections/gauges recorded as spec'd; both alert rules fire and resolve against a seeded registry.
- [ ] 5.5 Wording fix (nit e): replace "permissive-observe" in `gate.go` (and any other living artifact) — disabled state documented as **permissive-blind**; fail-permissive path documented as observed via the degradation counter.

## 6. Startup wiring, config default, enablement

- [ ] 6.1 `config.go` — flip `DefaultRiskLimits.Enabled` to `true`; update the documented-defaults comment (defaults stay permissive no-ops); adjust config tests.
- [ ] 6.2 Add sample `src/go/risk-overlay-config.yaml` (the resolved default path) with commented permissive defaults and `enabled: true`.
- [ ] 6.3 `cmd/main.go` — resolve config (invalid file fails startup with the sentinel; absent file → defaults), construct `GormCrowdingLookup` / `GormEvWeightLookup` / `GormSectorLookup`, build `SimulationRiskGate` with `BuildPortfolioSnapshot`, and `models.SetRiskGate(gate)` immediately after the kill-switch `SetOrderGate` block; set the `grodt.riskoverlay.enabled` gauge.
- [ ] 6.4 Integration-style test (fixture/smoke level): with default config, an ordinary Simulation order round-trips unchanged; kill switch engaged rejects before the overlay is consulted; Paper/Margin paths never invoke the gate.

## 7. Verification gates

- [ ] 7.1 `task test:portfolio-risk-overlay` — full riskoverlay suite green (engine fail-safe, ×100 mapping, seams, snapshot, telemetry recording, reduction-bypass invariant).
- [ ] 7.2 G1: `go build ./src/go/... ./cmd/...` — green.
- [ ] 7.3 G2: `task test` — green (backtester suite unaffected).
- [ ] 7.4 `task test:smoke` — green (server boots with the gate installed and default-enabled; real Simulation fill unchanged).
- [ ] 7.5 MANDATORY adversarial review (safety-critical, same bar as the parent change): confirms kill-switch-first composition, Simulation-only guard, reduction bypass ahead of all I/O, drawdown fail-safe boundaries, ×100 mapping, and every `>` vs `>=` comparison.
- [ ] 7.6 `openspec validate wire-risk-overlay-state --strict` — passes.

## 8. Operator-only follow-ups (NEVER autonomous)

- [ ] 8.1 Operator reviews and, if desired, narrows the shipped permissive limits in `risk-overlay-config.yaml` for real Simulation enforcement, and tunes the two new alert thresholds.
- [ ] 8.2 Operator decision (separate future change, explicit sign-off): any Paper/Margin gating. Out of scope here; the gate remains structurally Simulation-only.
