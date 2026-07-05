# Tasks — net-ev-cost-model

## 1. Package scaffold and cost parameters

- [x] 1.1 Create `src/go/tradingstack/costmodel/` package.
- [x] 1.2 `params.go` — `CostParams` struct (`CommissionPerTrade`, `CommissionPerShare`, `AvgSpreadBps`, `BorrowRateBpsAnnual`, `ImpactCoefficient`, `MaxImpactFractionOfEdge`), `DefaultCostParams()`, and `LoadCostParams(path string) (CostParams, error)` resolving `NET_EV_COST_CONFIG_PATH` → `${TRADING_PROJECT_DIR}/src/go/net-ev-cost-config.yaml` → built-in defaults.
- [x] 1.3 Add checked-in default config `src/go/net-ev-cost-config.yaml` alongside `options-config.yaml`, documenting each field with a comment.

## 2. Per-trade cost and gross/net EV calculation

- [x] 2.1 `trade_cost.go` — `TradeInput` struct and `TradeCost(trade, params) float64` with commission, spread, and (short-only) borrow components broken out as named intermediates.
- [x] 2.2 `ev.go` — `EVResult{GrossEV, NetEV float64}` (doc comments stating `NetEV` is the only decision-input field) and `ComputeEV(trades []TradeInput, params CostParams) EVResult`, classifying wins/losses off gross P&L for `GrossEV` and off net P&L for `NetEV`.
- [x] 2.3 `trade_cost_test.go` — hand-computed fixtures: long trade (no borrow), short trade with borrow accrual (see spec scenarios for exact expected values).
- [x] 2.4 `ev_test.go` — hand-computed 3-trade fixture demonstrating a gross-win/net-loss flip, plus an empty-trade-set zero-division-safe case.

## 3. Capacity estimate

- [x] 3.1 `capacity.go` — `EstimateCapacity(edgePerShare, price, avgDailyVolume float64, params CostParams) float64` implementing the square-root impact closed form; returns `0` for non-positive edge/price/ADV instead of erroring.
- [x] 3.2 `capacity_test.go` — hand-computed fixture (edge=2.00, price=50, ADV=1,000,000, impact_coefficient=0.1, max_impact_fraction_of_edge=0.2 → 6400 shares), the zero-edge case, and the ADV-doubling proportionality case.

## 4. Schema extension

- [x] 4.1 `migrate.go` — `MigrateNetEvCostModel(db *gorm.DB) error`: idempotent `ADD COLUMN IF NOT EXISTS` for `gross_ev`, `net_ev`, `capacity_shares` on `strategy_ev_weights`; returns an error if the table does not exist (i.e., `trading-stack-schema`'s migration has not run).
- [x] 4.2 `migrate_test.go` — testcontainers Postgres test: run `tradingstack.MigrateTradingStack`, insert a `strategy_ev_weights` row, run `MigrateNetEvCostModel`, assert the three new columns exist, the pre-existing row's other fields are unchanged, and running the migration twice is a no-op on the second call.

## 5. Taskfile wiring

- [x] 5.1 Add `test:net-ev-cost-model` target to `taskfile.yml` running `go test -count=1 ./src/go/tradingstack/costmodel/...`, exiting non-zero on any failure.

## 6. Verification and closeout

- [x] 6.1 Unit tests with hand-computed fixtures pass — `trade_cost_test.go`, `ev_test.go`, and `capacity_test.go` all green, asserting the exact numeric values worked out by hand in the spec scenarios (not values derived by running the implementation first).
- [x] 6.2 G1 — `go build ./src/go/... ./cmd/...` green.
- [x] 6.3 G2 — `task test` green (existing backtester-api suite unaffected; pre-existing failures in `backtester-api/models` are caused by a missing `.env` file in this worktree, reproduced identically with this change's files stashed out, and are unrelated to this change).
- [ ] 6.4 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time).
