# Overfitting Countermeasures (shared optimizer honesty gate)

## Why

The strategy- and scanner-optimizers are about to start producing config proposals from historical simulation data, and nothing today stops a proposal that merely memorized the past from looking excellent; this change adds the shared, deterministic gate — walk-forward/out-of-sample validation, parameter-stability checks, a deflated (multiple-testing-adjusted) performance bar, and minimum-sample gates — that every optimizer proposal must pass through before it can even be presented for operator review, with every verdict persisted for audit.

## What Changes

- Add a new deterministic Go library `src/go/tradingstack/overfitting/` that both optimizers submit proposal evidence to. It runs four checks in a fixed order over a caller-supplied `Evidence` struct and returns a single `Verdict`:
  1. **Minimum-sample gate** — the proposal's training data must contain at least a configurable minimum of decided labeled samples (default 500, matching the architecture doc's `min_labeled_samples`), and its out-of-sample window at least a configurable minimum (default 50).
  2. **Walk-forward / out-of-sample check** — evidence must carry at least a configurable minimum number of chronologically ordered walk-forward folds (default 3) whose test windows never precede their train windows (no lookahead), and out-of-sample performance must be positive and retain at least a configurable fraction (default 0.5) of in-sample performance.
  3. **Parameter-stability check** — proposed parameters may not move more than a configurable relative step (default 25%) from the current active baseline, and per-parameter values across walk-forward folds may not vary more than a configurable coefficient of variation (default 0.5).
  4. **Deflated performance check** — the out-of-sample performance t-statistic must clear a multiple-testing-adjusted bar that grows with the number of candidate parameterizations the optimizer tried (`max(minTStat, sqrt(2·ln(trials)))`), so a config that "won" a large search is held to a proportionally higher standard.
- Every check is a pure function; the gate is deterministic (same evidence + config → byte-identical verdict) and does no I/O, so the whole library is testable with synthetic fixtures.
- Persist every verdict (pass or fail) to a new Postgres table `overfitting_verdicts` via a dedicated additive migration entry point (`MigrateOverfittingCountermeasures`), keyed loosely by `proposal_id` + `proposal_kind` (`scanner` | `strategy`) so both optimizers share one audit trail. A `VerdictStore` interface with a GORM implementation and an in-memory fake keeps orchestration testable without a database.
- Add an operator self-check command `cmd/overfitting-check/main.go` (`--synthetic` mode, built-in passing and failing fixture evidence) with matching Taskfile targets `task optimizer:overfitting-check` and `task test:overfitting`.
- No changes to any existing table, model, migration, or the `optvalidation` package. No new metrics or alerts are added; any future ones must use the internal telemetry registry (`src/go/telemetry`) per ADR-0005. **Not BREAKING.**

## Capabilities

### New Capabilities

- `overfitting-countermeasures`: the shared, deterministic anti-overfitting gate (evidence contract, four checks, persisted verdicts, operator self-check) that strategy- and scanner-optimizer proposals must pass through before operator review.

### Modified Capabilities

(none)

## Impact

- New package: `src/go/tradingstack/overfitting/` (evidence/verdict types, four check files, gate orchestration, config with defaults, GORM verdict model + migration, store interface + fake, synthetic fixtures, tests). New thin command `cmd/overfitting-check/`. Only `taskfile.yml` is edited among existing files (adds `optimizer:overfitting-check` and `test:overfitting`).
- **Sequencing: this change is a dependency of both `scanner-optimizer` and `strategy-optimizer`** — their proposal flows submit evidence to this gate and persist its verdict id. It must land (or at least its spec must be signed off) before/alongside those changes; it imports nothing from either of them (the dependency points one way).
- Depends on `trading-stack-schema` only for the established `tradingstack` package conventions (`BaseModel`, additive migration pattern); it reads and writes no `trading-stack-schema` table.
- Postgres persistence only — ESDB is untouched (ADR-0004 pending). No new external dependencies beyond already-vendored modules (`gorm.io/gorm`, `github.com/google/uuid`, `github.com/stretchr/testify`).
- Verification is local-only: deterministic unit tests over synthetic fixtures plus a testcontainers round-trip for the verdict table. No prod DB, no live/paper orders, no push, no new infra.
- **Conservative scope decisions needing sign-off** (detail in design.md): the deflated-performance check is a multiple-testing-adjusted t-statistic bar, not the full Bailey–López de Prado Deflated Sharpe Ratio (which needs per-return skew/kurtosis the evidence contract does not yet carry); default thresholds (500 samples, 3 folds, 0.5 OOS retention, 25% max step, CV 0.5, min t-stat 2.0) are starting points the operator can tune via the gate config, not tuned constants.
