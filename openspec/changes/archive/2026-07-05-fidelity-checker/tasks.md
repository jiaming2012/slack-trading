# Tasks — fidelity-checker

## 1. Package scaffold and input types

- [x] 1.1 Create `src/go/tradingstack/fidelity/` package with `errors.go` (sentinel errors incl. `ErrNoLiveTrades`) and `trade.go` (`Trade`, `TradeSet`, `Period` value types; `ExitReason` reusing the `trading-stack-schema` vocabulary `stop | target | timeout | signal_exit`).

## 2. Deterministic pairing

- [x] 2.1 `pair.go` — `PairTrades(live, sim []Trade) ([]Pair, Unmatched)` keyed on `(StrategyID, Symbol, EntryAt)`, sorting keys before matching so input order never affects the result; return unmatched live/sim trades separately and exclude them from all downstream drift.

## 3. Per-pair comparison

- [x] 3.1 `compare.go` — `ComparePair(p Pair) PairDelta` computing `PnLDelta` (sim minus live), `FillDelta`, `HoldDelta`, and `ExitReasonMatch`, as a pure function (no clock, randomness, DB, or network).

## 4. Composite drift score

- [x] 4.1 `score.go` — `FidelityConfig` (component weights summing to 1, normalization constants, `ToleranceThreshold`) with `DefaultConfig()` (`ToleranceThreshold = 0.2`), and `Score(deltas []PairDelta, cfg FidelityConfig) Composite` producing aggregated `drift_pnl`, averaged `drift_fill`, and the composite `drift_score` normalized and clamped to `[0.0, 1.0]`, monotonic non-decreasing in each dimension.

## 5. Per-strategy aggregation and record mapping

- [x] 5.1 `aggregate.go` — `Aggregate(pairs []Pair, period Period, cfg FidelityConfig) []Result` grouping by `StrategyID`, one `Result` per strategy with `drift_pnl`, `drift_fill`, `drift_score`, `within_tolerance` (`drift_score <= ToleranceThreshold`).
- [x] 5.2 `Result.ToRecord()` mapping field-for-field onto `tradingstack.SimulatorFidelity` (`strategy_id`, `period_start`, `period_end`, `drift_pnl`, `drift_fill`, `drift_score`, `within_tolerance`).

## 6. Fidelity gate and alert

- [x] 6.1 `gate.go` — `Decision` enum (`Proceed | Pause`), `Alerter` interface (`RaiseFidelityAlert(ctx, Result)`), `Gate(results []Result, a Alerter) map[string]Decision` (Proceed when `within_tolerance`, else Pause + raise exactly one alert per breaching strategy; never stops an optimizer itself), and a `LogAlerter` default backed by logrus (OTel-bridged via existing `otellogrus`).

## 7. Checker orchestration and persistence

- [x] 7.1 `checker.go` — `RunFidelityCheck(ctx, set TradeSet, period Period, cfg FidelityConfig, a Alerter) ([]Result, map[string]Decision, error)` orchestrating pair → compare → aggregate → gate; return `ErrNoLiveTrades` cleanly when the live set is empty. Add `Persist(db, results)` writing `SimulatorFidelity` rows.
- [x] 7.2 `synthetic.go` — `SyntheticTradeSet()` builders (zero-drift, fixed-PnL-drift, exit-reason-mismatch, extreme-drift) with drift known by construction, shared by tests and the command's `--synthetic` mode.

## 8. Operator entry point and Taskfile target

- [x] 8.1 `cmd/fidelity-check/main.go` — flags `--period-start`, `--period-end`, `--synthetic`; runs `RunFidelityCheck`, prints per-strategy `drift_score` / `within_tolerance` / decision; exits non-zero only on error (a tolerance breach exits zero); reports "no live trades available" and exits zero on `ErrNoLiveTrades`.
- [x] 8.2 Add `fidelity:check` target to `taskfile.yml` invoking the command (default `--synthetic` for a runnable no-input self-check), exiting non-zero only on execution error.

## 9. Unit tests (synthetic known-drift pairs)

- [x] 9.1 `fidelity_test.go` — pairing determinism: same pairs regardless of input order; unmatched trades reported and excluded from drift.
- [x] 9.2 Per-pair comparison: zero-drift pair → all deltas zero and exit-reason match true; known PnL drift → `PnLDelta` equals the constructed sim-minus-live value; exit-reason mismatch flagged.
- [x] 9.3 Scoring: perfect fidelity → `drift_score 0.0`; extreme-drift set → `drift_score` clamped to `1.0`; monotonicity across two strategies of increasing drift.
- [x] 9.4 Aggregation and mapping: two strategy ids → two `Result`s each with own drift; `within_tolerance` reflects threshold; `Result.ToRecord()` maps field-for-field onto `tradingstack.SimulatorFidelity`.
- [x] 9.5 Gate: within-tolerance → Proceed, spy `Alerter` not invoked; breach → Pause + spy `Alerter` invoked exactly once; mixed batch → one Proceed, one Pause, exactly one alert total.

## 10. Verification and closeout

- [x] 10.1 Unit tests with synthetic sim-vs-live pairs of known drift pass: `go test -count=1 ./src/go/tradingstack/fidelity/...` green (pairing, comparison, scoring/clamping/monotonicity, aggregation/mapping, gate/alert).
- [x] 10.2 G1 — `go build ./src/go/... ./cmd/...` green (new package and `cmd/fidelity-check` compile, nothing else broken).
- [x] 10.3 G2 — `task test` green (existing backtester-api suite unaffected).
- [x] 10.4 G6 — Fable adversarial review approves the diff (deterministic pairing/scoring, correct sim-minus-live sign on `drift_pnl`, `drift_score` clamping and monotonicity, gate raises exactly one alert per breach and never stops an optimizer, no leakage into playground tables) before commit.
- [x] 10.5 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time).
