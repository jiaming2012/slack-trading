# Design — net-ev-cost-model

## Approach

A pure-calculation Go package (`costmodel`) that turns closed trades into cost-adjusted EV, plus a small additive migration that gives the cost-adjusted numbers a place to live on the existing `strategy_ev_weights` table. No new service, no new RPC surface, no background job — this is a library that a future `ev-tracker` aggregation pipeline calls. Everything here is deterministic arithmetic, so verification is unit tests against hand-computed fixtures (the correct answer is known before the code is written), not integration behavior.

The two capabilities split cleanly along Finding 7's two recommendations: `net-ev-cost-model` (per-trade cost + gross/net EV) and `strategy-capacity-estimate` (market-impact-based position sizing). Both share one `CostParams` struct and one config file because an operator tunes them together (they are all "how expensive is trading this strategy" assumptions).

## Package and file layout

New package `src/go/tradingstack/costmodel/` (import path `github.com/jiaming2012/slack-trading/src/go/tradingstack/costmodel`):

- `params.go` — `CostParams` struct (`CommissionPerTrade`, `CommissionPerShare`, `AvgSpreadBps`, `BorrowRateBpsAnnual`, `ImpactCoefficient`, `MaxImpactFractionOfEdge`), `DefaultCostParams()`, and `LoadCostParams(path string) (CostParams, error)` — resolves `NET_EV_COST_CONFIG_PATH` env var, else `${TRADING_PROJECT_DIR}/src/go/net-ev-cost-config.yaml`, else built-in defaults if the file is absent.
- `trade_cost.go` — `TradeInput` struct (`Side` `"long"|"short"`, `EntryPrice`, `ExitPrice`, `Quantity`, `HoldDays`), `TradeCost(trade TradeInput, params CostParams) (total float64)`, with commission/spread/borrow broken out as named intermediate values for testability.
- `ev.go` — `EVResult` struct (`GrossEV`, `NetEV` — both documented: only `NetEV` is a decision input), `ComputeEV(trades []TradeInput, params CostParams) EVResult`.
- `capacity.go` — `EstimateCapacity(edgePerShare, price, avgDailyVolume float64, params CostParams) float64`.
- `migrate.go` — `MigrateNetEvCostModel(db *gorm.DB) error`, additive `ALTER TABLE strategy_ev_weights ADD COLUMN IF NOT EXISTS gross_ev NUMERIC`, same for `net_ev`, `capacity_shares`. Idempotent; errors if `strategy_ev_weights` does not yet exist (i.e., `trading-stack-schema`'s migration must run first).
- `net-ev-cost-config.yaml` (in `src/go/`, sibling to `options-config.yaml`) — checked-in default values, editable per-environment like the options config.
- Test files: `trade_cost_test.go`, `ev_test.go`, `capacity_test.go` (pure unit tests, no DB), `migrate_test.go` (a lightweight testcontainers Postgres test: run `trading-stack-schema`'s `MigrateTradingStack`, then this change's `MigrateNetEvCostModel`, assert the three columns exist and existing data survives).

## Data flow

1. A caller (eventually `ev-tracker`) assembles `[]TradeInput` from `tradingstack.SimOutcome` rows (or live trade records) for a strategy/regime window.
2. `ComputeEV` returns `EVResult{GrossEV, NetEV}`.
3. `EstimateCapacity` is called separately with the strategy's current edge-per-share (derived from `NetEV` per share) and volume/price context.
4. The caller writes `GrossEV`, `NetEV`, and the capacity result into the corresponding `strategy_ev_weights` row's `gross_ev`, `net_ev`, `capacity_shares` columns (added by this change's migration).
5. `ev-tracker`'s `ev_slope` / `ev_weight` / retirement logic reads `net_ev` exclusively — `gross_ev` is carried only so an operator or dashboard can see how much cost is eroding a strategy's edge.

## Field mapping decisions

- **Cost basis for spread**: spread cost uses `entry_price`, not an average of entry/exit, to keep the formula a pure function of inputs already on a closed trade and to avoid a second price lookup. This is a stated approximation (spread cost is estimated once at trade-cost time, not measured against the actual realized spread at fill).
- **Borrow day-count**: `hold_days / 360` (banker's/Actual-360 convention) rather than `/365`, chosen for arithmetic cleanliness in the fixtures and consistency with typical broker margin/borrow day-count conventions. This is a documented estimate, not a claim of matching any specific broker's exact accrual schedule.
- **Win/loss classification uses net P&L for NetEV, gross P&L for GrossEV** — this is the crux of Finding 7: a trade can be a gross winner and a net loser. Classifying both EVs off the same (net) P&L would understate how much cost erosion is happening; computing GrossEV oblivious to cost is what lets the two numbers be compared meaningfully.
- **Capacity formula** is a square-root market-impact model (a standard simplification of the Almgren-Chriss family), not a calibrated Kyle's-lambda or venue-specific model — appropriate for "a simple capacity estimate" per Finding 7's own framing. `ImpactCoefficient` is a tunable placeholder an operator can calibrate later; no claim is made that the default value is empirically fitted to this platform's actual fills.

## Out of scope

- `ev-tracker`'s rolling-window aggregation (`ev_30d`, `ev_90d`, `ev_slope` computation, decay detection, the quarterly EV-weight feedback loop into the scanner optimizer). This change provides the cost engine and the columns `ev-tracker` will populate and read; wiring `ComputeEV`/`EstimateCapacity` into that pipeline is `ev-tracker`'s own task list.
- Any live measurement of actual realized spread, actual commission invoices, or actual broker borrow rates — all three are estimated from configured assumptions, not reconciled against real broker statements. A future fidelity-checker-style reconciliation is a candidate follow-up, not part of this change.
- Wiring `MigrateNetEvCostModel` into server startup (`cmd/main.go`) or any RPC/REST surface — mirrors `trading-stack-schema`'s decision to keep v4 migrations explicitly invoked, not startup-wired.
- Any UI/dashboard rendering of gross vs. net EV or capacity — this change is calculation and storage only.
- Per-symbol or per-asset-class cost parameter overrides (e.g., different spread assumptions for large-cap vs. small-cap) — the config is a single global `CostParams` for this change; segmentation is a future extension.

## Dependency ordering within the batch

- **Requires `trading-stack-schema` to have already merged**: `MigrateNetEvCostModel` alters a table that migration creates, and `TradeInput` fixtures in tests are built from data shaped like `tradingstack.SimOutcome`. This change must be authored and tested against a tree that already has the `tradingstack` package.
- **Interface dependency on `ev-tracker`** (not a build dependency): `ev-tracker` is expected to import `costmodel` and call `ComputeEV`/`EstimateCapacity` when computing its rolling EV windows. This change does not import anything from `ev-tracker` and does not block on `ev-tracker` existing yet — it can land independently as long as `trading-stack-schema` is in place. If `ev-tracker` lands first without this change, its EV values are gross-only until this change lands and is wired in; once both are landed, `ev-tracker`'s own follow-up task is to switch its decision logic to `net_ev`.
- No dependency on any Tier B or Tier C card in the overnight batch.

## Verification gates

- **Unit tests with hand-computed fixtures** — `trade_cost_test.go` and `ev_test.go` assert exact numeric outputs (see the spec scenarios) computed by hand before the implementation exists, not derived from running the code and pinning its output.
- **G1** — `go build ./src/go/... ./cmd/...` green.
- **G2** — `task test` green (existing backtester-api suite unaffected; new `costmodel` package tests run under `go test ./...` scoping used by `task test:net-ev-cost-model`).
- A local-Postgres migration test (`migrate_test.go`, testcontainers) is included as an implementation-quality bar for the schema-extension requirement, but is not one of this change's named overnight gates — `task test:net-ev-cost-model` running the full package suite (pure-calc unit tests plus the migration test) is sufficient to close out this change.

## Deferred validation

None (Tier A). Every requirement in both capability specs is verifiable overnight with local unit tests and a local-Postgres migration check — no live feed, no live/paper broker order, no external infrastructure, and no dependence on accumulated production data is required to validate this change.
