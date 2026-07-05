# EV Tracker (expected-value computation engine)

## Why

Before the system can trust or retire a strategy, it needs a deterministic way to measure each strategy's expected value per market regime and detect whether that edge is growing or decaying. This change delivers that computation engine and its persistence into `strategy_ev_weights`, driven by synthetic trade fixtures so it is fully testable overnight; the live trade feed that will drive it accrues later.

## What Changes

- Add a new Go package that computes expected value (EV) per strategy per regime from a set of closed-trade outcomes, using the documented formula `EV = (win_rate × avg_win) − (loss_rate × avg_loss)`.
- Compute EV over three rolling windows relative to a caller-supplied as-of timestamp: 30-day, 90-day, and all-time.
- Compute an EV trend slope by ordinary-least-squares regression over per-bucket EV values, and classify each strategy's edge as improving, stable, or decaying (decay detection).
- Populate `strategy_ev_weights` with the documented weight logic: slope > 0.1 → weight 1.0 (scale up), 0 ≤ slope ≤ 0.1 → weight 0.7 (stable), slope < 0 → weight 0.3 (flag for review); retired strategies produce **no** row (excluded, effective weight 0).
- Rank strategies by EV as a computed output.
- Read closed-trade outcomes through a repository interface so the data source is pluggable — synthetic fixtures now, a live-trade-backed source later.
- Add a `task test:ev-tracker` Taskfile target that runs the unit and testcontainers suite.
- No changes to any existing playground table, model, or migration path. Not BREAKING.

## Capabilities

### New Capabilities

- `ev-computation`
- `ev-persistence`

### Modified Capabilities

(none)

## Impact

- New package `src/go/evtracker/` (computation engine, persistence, repository interface, errors, tests). Only `taskfile.yml` is modified among existing files (adds `test:ev-tracker`).
- Depends on `trading-stack-schema`: writes rows through that change's `tradingstack.StrategyEvWeight` model and `strategy_ev_weights` table. This change must be authored after `trading-stack-schema` merges.
- Dependency surface uses already-vendored modules only: `gorm.io/gorm`, `github.com/google/uuid`, `github.com/testcontainers/testcontainers-go`, `github.com/stretchr/testify`. No new external infrastructure.
- The EV formula here is gross (pre-cost). Substituting a net-of-cost EV is owned by the separate `net-ev-cost-model` change and is out of scope here.
- Verification is local-only (unit tests + testcontainers Postgres). No prod DB, no live/paper broker, no infra provisioning.
