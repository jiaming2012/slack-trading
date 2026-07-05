# Fidelity Checker (simulator vs. live trust gate)

## Why

The optimizers are only as trustworthy as the simulator: if the simulator drifts from live execution, every optimizer decision downstream is built on sand. The architecture doc is explicit — build the fidelity checker *before* the optimizers, because it is the trust gate that decides whether optimizer training may proceed at all.

## What Changes

- Add a new deterministic Go sub-package `src/go/tradingstack/fidelity/` that compares a set of live trades for a period against a re-run of the simulator over the same period.
- Compare each matched sim-vs-live trade pair on the four dimensions the architecture doc names: **PnL, fill prices, hold time, exit reason**.
- Compute a per-strategy composite drift result: `drift_pnl` (sim PnL minus live PnL), `drift_fill` (average fill-price delta), `drift_score` (composite in `[0.0, 1.0]`, 0 = perfect fidelity), and `within_tolerance` (drift_score vs. a configurable tolerance threshold).
- Persist each per-strategy result as a `simulator_fidelity` record using the `tradingstack.SimulatorFidelity` model delivered by `trading-stack-schema`.
- Expose a **fidelity gate signal**: within tolerance → optimizers proceed; tolerance exceeded → optimizers pause and an alert is raised through the existing logging/OTel path (logrus, already OTel-bridged). The gate emits a decision; it does not itself stop any optimizer (consumers act on the signal).
- Add an operator-visible entry point and Taskfile target `task fidelity:check` that runs the checker for a period. Overnight scope wires the weekly-scheduled invocation and validates the engine with **synthetic sim-vs-live pairs whose drift is known by construction**; ingestion of real live trades and the re-run of the real simulator accrue in a later change.
- No changes to any existing playground table, model, or migration, and no changes to the `tradingstack` schema models. **Not BREAKING.**

## Capabilities

### New Capabilities

- `fidelity-checker`

### Modified Capabilities

(none)

## Impact

- New sub-package: `src/go/tradingstack/fidelity/` (pairing, comparison, scoring, aggregation, gate, checker orchestration, errors, tests). New thin command `cmd/fidelity-check/` for the Taskfile entry point. Only `taskfile.yml` is edited among existing files (adds `fidelity:check`).
- **Depends on `trading-stack-schema`**: imports `tradingstack.SimulatorFidelity` (and its exit-reason vocabulary) and must be authored after that change lands and archives. No other batch change is a prerequisite.
- New dependency surface only within already-vendored modules: `gorm.io/gorm`, `github.com/google/uuid`, `github.com/sirupsen/logrus` (+ the existing `otellogrus` bridge). No new external infrastructure, no scheduler/cron, no prod DB, no live/paper broker.
- Downstream: the optimizer changes (`optimizer-validation-pipeline` and later strategy/scanner optimizers) consume the fidelity gate signal and the `simulator_fidelity` rows this change writes. This change is the trust gate they build on.
