# Design — fidelity-checker

## Approach

A deterministic, pure-Go comparison-and-scoring engine that takes two sets of trades for the same period — live trades and the simulator's re-run over that period — pairs them, computes per-pair deltas on the four architecture-named dimensions (PnL, fill price, hold time, exit reason), reduces those into a per-strategy composite `drift_score` in `[0.0, 1.0]`, decides `within_tolerance`, persists a `simulator_fidelity` row per strategy, and emits a gate signal (Proceed / Pause) that raises an alert on breach through the existing logging/OTel path.

The engine is deliberately I/O-free at its core: pairing, comparison, scoring, and gating are pure functions over in-memory structs. This is what makes the overnight verification honest — synthetic sim-vs-live pairs with drift known by construction feed the pure functions and the outputs are checked against hand-computed expectations, with no database, clock, randomness, or network in the loop. Persistence and the operator command are thin shells around the pure core.

## Package and file layout

New sub-package `src/go/tradingstack/fidelity/` (package `fidelity`, import path `github.com/jiaming2012/slack-trading/src/go/tradingstack/fidelity`):

- `trade.go` — input value types `Trade` (fields: `StrategyID`, `Symbol`, `EntryAt`, `EntryFill`, `ExitFill`, `PnL`, `HoldDuration`, `ExitReason`) and `TradeSet` wrapping live and simulator slices for a period. `ExitReason` reuses the vocabulary from `trading-stack-schema` (`stop | target | timeout | signal_exit`). Defined locally so real live-trade ingestion can wire into it later without changing the engine.
- `pair.go` — `PairTrades(live, sim []Trade) ([]Pair, Unmatched)` deterministic matcher keyed on `(StrategyID, Symbol, EntryAt)`; sorts keys before matching so ordering never affects the result.
- `compare.go` — `ComparePair(p Pair) PairDelta` computing `PnLDelta`, `FillDelta`, `HoldDelta`, `ExitReasonMatch`.
- `score.go` — `FidelityConfig` (weights + `ToleranceThreshold`, with a `DefaultConfig()`), and `Score(deltas []PairDelta, cfg FidelityConfig) Composite` producing the normalized-and-clamped `drift_score` plus the aggregated `drift_pnl` and averaged `drift_fill`.
- `aggregate.go` — `Aggregate(pairs []Pair, period Period, cfg FidelityConfig) []Result` grouping by `StrategyID` and building one `Result` per strategy; `Result.ToRecord()` maps onto `tradingstack.SimulatorFidelity`.
- `gate.go` — `Decision` enum (`Proceed | Pause`), `Alerter` interface (`RaiseFidelityAlert(ctx, Result)`), `Gate(results []Result, a Alerter) map[string]Decision`, and a `LogAlerter` default backed by logrus (already OTel-bridged via `otellogrus` per project setup).
- `checker.go` — `RunFidelityCheck(ctx, set TradeSet, period Period, cfg FidelityConfig, a Alerter) ([]Result, map[string]Decision, error)` orchestrating pair → compare → aggregate → gate; plus `Persist(db, results)` writing `SimulatorFidelity` rows.
- `synthetic.go` — `SyntheticTradeSet()` builders producing sim-vs-live pairs whose drift is known by construction (zero-drift set, fixed-PnL-drift set, exit-reason-mismatch set, extreme-drift set). Shared by the tests and the `--synthetic` command mode so the overnight self-check and the unit tests exercise identical data.
- `errors.go` — sentinel errors (e.g. `ErrNoLiveTrades`).
- `fidelity_test.go` — table-driven unit tests over the synthetic sets.

New thin command `cmd/fidelity-check/main.go` — flags `--period-start`, `--period-end`, `--synthetic`; calls `RunFidelityCheck`, prints per-strategy `drift_score` / `within_tolerance` / decision, exits non-zero only on error. `--synthetic` runs `SyntheticTradeSet()` so the command is runnable with no live-trade input.

## Data flow

```
live trades (period)  ─┐
                       ├─► PairTrades ─► ComparePair (per pair) ─► Aggregate (per strategy)
sim re-run (period)   ─┘                                                │
                                                                        ▼
                                              Result{drift_pnl, drift_fill, drift_score, within_tolerance}
                                                        │                         │
                                          Persist → simulator_fidelity      Gate → Proceed / Pause
                                                                                  │ (Pause)
                                                                        Alerter.RaiseFidelityAlert (logrus/OTel)
```

## Scoring decision

`drift_score` is a fixed weighted sum of three normalized components — PnL drift, fill drift, and exit-reason mismatch rate — each squashed into `[0, 1]` (bounded normalization, e.g. `x/(x+k)` for the magnitude components and a direct `[0,1]` mismatch fraction), then combined with weights that sum to 1 and clamped to `[0, 1]`. Consequences guaranteed by construction and asserted in tests: perfect fidelity → `0.0`; arbitrarily large drift → clamped to `1.0`; monotonic non-decreasing in each dimension. `within_tolerance = drift_score <= cfg.ToleranceThreshold` (default `0.2`). Exact weights and `k` constants are config with documented defaults; they are deterministic, not tuned against live data (which does not exist yet).

## Out of scope

- Ingesting real live trades and running the real simulator re-run over a period. This change wires the entry point and validates the engine on synthetic input; real input ingestion accrues in a later change once live trades exist.
- The actual weekly scheduler / cron / Temporal workflow. `task fidelity:check` is the wired, runnable invocation; scheduling it is out of scope (no new infra overnight).
- Continuous / per-trade near-real-time fidelity (gap-analysis Finding 6) — this change delivers the weekly batch engine only.
- Enforcing the pause on any optimizer. The gate emits the Proceed/Pause signal and raises the alert; optimizer changes consume it.
- Any change to `tradingstack` schema models, playground tables, or `dbutils`.
- Tuning the drift weights against live data (impossible before live trades accrue).

## Dependency ordering within the batch

Depends on `trading-stack-schema` (imports `tradingstack.SimulatorFidelity` and the exit-reason vocabulary); MUST be authored after that change merges and archives. No other batch change is a prerequisite. Downstream, the optimizer changes consume the fidelity gate signal and the `simulator_fidelity` rows this change writes — this is the "build before the optimizers" trust gate.

## Verification gates

- **Unit tests with synthetic sim-vs-live pairs of known drift** — `go test ./src/go/tradingstack/fidelity/...`: zero-drift → `drift_score 0.0` and Proceed; known fixed PnL drift → `drift_pnl` equals the constructed value; exit-reason-mismatch set → flagged and reflected in the score; extreme-drift set → `drift_score` clamped to `1.0`, `within_tolerance` false, Pause, and exactly one alert raised via a spy `Alerter`; per-strategy aggregation yields one `Result`/record per strategy id; `Result.ToRecord()` field-for-field maps onto `tradingstack.SimulatorFidelity`.
- **G1** — `go build ./src/go/... ./cmd/...` green (new package and command compile, nothing else broken).
- **G2** — `task test` green (existing backtester-api suite unaffected).
- **G6** — Fable adversarial review of the diff before commit (determinism of pairing/scoring, correct sim-minus-live sign on `drift_pnl`, clamping and monotonicity of `drift_score`, gate raises exactly one alert per breach and never stops an optimizer itself, no leakage into playground tables).

## Deferred validation

Per the overnight run plan, the following are explicitly deferred and NOT attempted overnight:

- End-to-end validation against **real live trades** and a **real simulator re-run** — real live-trade input accrues later; overnight validation is limited to synthetic pairs whose drift is known by construction.
- The **weekly scheduled** invocation firing on a real cadence (no scheduler/cron/infra provisioning overnight; the `task fidelity:check` entry point is wired and runnable, scheduling it is deferred).
- Persistence is exercised as a field-for-field mapping (`Result.ToRecord()`) unit test; a full testcontainers Postgres write→read of `simulator_fidelity` rides on the round-trip suite already delivered by `trading-stack-schema` and is not re-verified here.
