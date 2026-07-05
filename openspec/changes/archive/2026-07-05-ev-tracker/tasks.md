# Tasks — ev-tracker

## 1. Package scaffold and inputs

- [x] 1.1 Create `src/go/evtracker/` with `trade_outcome.go` (`TradeOutcome` struct, `TradeOutcomeRepository` interface, and an in-memory fixture implementation), `options.go` (`Options{ BucketDays, Retired }`, default bucket 30d), and `errors.go` (sentinels).

## 2. Computation core (pure, deterministic)

- [x] 2.1 `ev.go` — `ComputeEV(trades) EVStats` implementing `EV = win_rate·avg_win − loss_rate·avg_loss`; win = pnl>0, loss = pnl<0, breakeven (pnl==0) excluded from decided; `avg_loss` is a positive magnitude; `win_rate + loss_rate = 1` over decided trades.
- [x] 2.2 `window.go` — rolling-window filters for 30d / 90d / all-time relative to a supplied as-of timestamp (no wall-clock reads); trades after as-of excluded from all windows.
- [x] 2.3 `slope.go` — partition a group's trades into consecutive equal-length buckets (default 30d), compute per-bucket EV for non-empty buckets, and OLS-fit the slope over integer bucket index; return null/undefined when fewer than two non-empty buckets.
- [x] 2.4 `weights.go` — band constants and `ClassifyWeight(slope)`: slope>0.1 → 1.0/improving; 0≤slope≤0.1 → 0.7/stable; slope<0 → 0.3/decaying; null → 0.7/insufficient-data; boundaries 0.1 and 0.0 inclusive in the stable band.
- [x] 2.5 `ranking.go` — `RankByEV` descending on all-time EV, tie-break by strategy_id then regime.
- [x] 2.6 `compute.go` — `Compute(trades, asOf, opts)` grouping by `(strategy_id, regime)`, assembling `StrategyEVResult{ ev_30d, ev_90d, ev_all_time, ev_slope *float64, ev_weight, status, rank }`.

## 3. Persistence (writes strategy_ev_weights)

- [x] 3.1 `persist.go` — `Recompute(ctx, db, repo, asOf, opts)`: fetch via repository, drop retired strategy ids before compute, run `Compute`, and write one `tradingstack.StrategyEvWeight` row per result with `computed_at = asOf`; store null slope as SQL NULL.

## 4. Unit tests (hand-computed fixtures)

- [x] 4.1 `ev_test.go` — EV formula fixture (6 winners @ +2.0, 4 losers @ −1.0 → EV 0.8), breakeven-exclusion case, and per-`(strategy,regime)` independence.
- [x] 4.2 `slope_test.go` — window boundary cases (45-day trade excluded from ev_30d only; post-as-of trade excluded everywhere) and OLS slope on the linear series 0.0/0.2/0.4/0.6 → slope 0.2, plus single-bucket → null slope.
- [x] 4.3 `weights_test.go` — every band including exact 0.1 and 0.0 boundaries (→0.7), slope 0.2 (→1.0), slope −0.05 (→0.3), null (→0.7/insufficient-data); and ranking order 0.8/0.1/−0.3 → ranks 1/2/3.
- [x] 4.4 Determinism test — same fixture + as-of computed twice yields identical results.

## 5. Testcontainers persistence tests

- [x] 5.1 `evtracker_test.go` — testcontainers Postgres helper; migrate via `tradingstack.MigrateTradingStack`; run `Recompute` with the in-memory fixture repo; assert exactly one row per group with per-field equality to computed values.
- [x] 5.2 Persistence edge tests — null slope persists as SQL NULL with weight 0.7; a retired strategy produces zero rows; a second recompute for the same as-of with changed inputs yields latest-`computed_at` rows reflecting the new inputs.

## 6. Taskfile wiring

- [x] 6.1 Add `test:ev-tracker` target to `taskfile.yml` running `go test -count=1 ./src/go/evtracker/...`, exiting non-zero on any failure.

## 7. Verification and closeout

- [x] 7.1 Unit + testcontainers tests pass: `task test:ev-tracker` green (formula, windows, slope, all weight bands, ranking, persistence, retired-exclusion, re-run).
- [x] 7.2 G1 — `go build ./src/go/... ./cmd/...` green.
- [x] 7.3 G2 — `task test` green (existing backtester-api suite unaffected).
- [x] 7.4 G6 — Fable adversarial review approves the diff (formula faithfulness, band-boundary correctness, determinism, no leakage into playground or cost logic) before commit.
- [x] 7.5 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time).
