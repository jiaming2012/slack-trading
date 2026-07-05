# Net EV Cost Model

## Why

The EV formula (`win_rate × avg_win − loss_rate × avg_loss`) ignores commissions, spread, and short borrow costs, so a strategy can show EV-positive while it is actually net-negative after real trading costs — and nothing today estimates how much size a strategy can carry before its own market impact eats the edge.

## What Changes

- Add a `costmodel` calculation package that computes, per trade: commission cost, estimated spread cost, and borrow cost (shorts only), then rolls trades up into a `GrossEV` (cost-blind) and a `NetEV` (cost-adjusted) per strategy/regime — using the **net** classification (win/loss determined by post-cost P&L) so a trade that is gross-profitable but net-unprofitable is correctly counted as a net loss.
- Add a configurable cost-parameters model (commission tier, average spread assumption in bps, annualized short borrow rate) loaded from a YAML file with built-in defaults, following the existing `options-config.yaml` convention.
- Add a capacity estimate: the maximum position size (shares) at which estimated square-root market impact exceeds a configurable fraction `X` of expected per-share edge.
- Extend the `strategy_ev_weights` table (owned by `trading-stack-schema`) with additive, idempotent `gross_ev`, `net_ev`, and `capacity_shares` columns via this change's own migration function — `trading-stack-schema`'s models and migration are not modified.
- Establish the contract that all downstream EV-weight and retirement-signal consumers (the future `ev-tracker` aggregation logic) MUST read `net_ev`, never `gross_ev`, for decisions — `gross_ev` is retained only for display/diagnostics.
- Add a `task test:net-ev-cost-model` Taskfile target running the package's unit tests.
- No existing package, table, or migration is modified. **Not BREAKING.**

## Capabilities

### New Capabilities

- `net-ev-cost-model`
- `strategy-capacity-estimate`

### Modified Capabilities

(none)

## Impact

- New package: `src/go/tradingstack/costmodel/` (cost params, trade cost, gross/net EV, capacity estimate, migration, unit tests) plus a new default config file `src/go/net-ev-cost-config.yaml`. Only `taskfile.yml` is touched outside the new package (adds `test:net-ev-cost-model`).
- Depends on `trading-stack-schema`: this change imports and extends the `StrategyEvWeight` / `SimOutcome` shapes that change defines and requires its migration to have already created `strategy_ev_weights` before this change's own migration runs. Must be authored and land after `trading-stack-schema` merges.
- Depends on `ev-tracker` at the interface level only: `ev-tracker` is expected to call this package's `ComputeEV` and read the `gross_ev`/`net_ev`/`capacity_shares` columns this change adds when computing `ev_slope`, `ev_weight`, and retirement signals. Wiring that call is owned by `ev-tracker`, not this change — this change ships the cost engine and schema columns as a library, not the rolling-window aggregation itself.
- No new external infrastructure, no live/paper broker calls, no prod DB access. All verification is local (unit tests plus a local-Postgres migration check).
