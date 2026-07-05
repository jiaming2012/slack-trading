# Design — ev-tracker

## Approach

A self-contained Go package `src/go/evtracker/` that (1) computes expected value per strategy per regime as a pure deterministic function and (2) persists the results into the `strategy_ev_weights` table delivered by `trading-stack-schema`. The computation core reads no clock, no DB, and no randomness — everything derives from a supplied slice of closed-trade outcomes plus a caller-supplied as-of timestamp — which is exactly what makes it testable with synthetic fixtures whose EV and slope are known analytically. Persistence is a thin layer that pulls trades through a repository interface and writes the computed rows; tests drive it with an in-memory fixture repository against a testcontainers Postgres. Nothing in the existing `backtester-api` playground path is touched.

### Analytic identity used to build fixtures

With `win_rate = wins/decided`, `loss_rate = losses/decided`, `avg_win = Σwin_pnl/wins`, `avg_loss = Σ|loss_pnl|/losses`, the formula `EV = win_rate·avg_win − loss_rate·avg_loss` reduces to `(Σwin_pnl − Σ|loss_pnl|)/decided` = the mean pnl of decided (non-breakeven) trades. This identity lets every fixture's EV be hand-computed by averaging the decided-trade pnls, and every slope fixture be built as a linear per-bucket EV series with a known OLS slope.

## Package and file layout

New package `src/go/evtracker/` (package `evtracker`, import path `github.com/jiaming2012/slack-trading/src/go/evtracker`):

- `trade_outcome.go` — `TradeOutcome{ StrategyID string; Regime string; ClosedAt time.Time; PnL float64 }` and the `TradeOutcomeRepository` interface (`FetchTradeOutcomes(ctx, asOf time.Time) ([]TradeOutcome, error)`), plus an in-memory implementation used by tests.
- `ev.go` — `ComputeEV(trades []TradeOutcome) EVStats` returning win_rate, avg_win, loss_rate, avg_loss, ev, and decided/win/loss/breakeven counts. Win = pnl>0, loss = pnl<0, breakeven = pnl==0 excluded from decided.
- `window.go` — window filtering: `Within(trades, asOf, days)` and an `AllTime` variant.
- `slope.go` — bucketing into equal-length time buckets (default 30d) and `OLSSlope(points []float64) (float64, bool)`; returns `ok=false` when fewer than two points.
- `weights.go` — band constants (`SlopeScaleUpThreshold = 0.1`; weights `1.0 / 0.7 / 0.3`) and `ClassifyWeight(slope *float64) (weight float64, status DecayStatus)`; statuses `improving | stable | decaying | insufficient_data`.
- `ranking.go` — `RankByEV(results)` descending on all-time EV, tie-break by strategy_id then regime.
- `compute.go` — `Compute(trades []TradeOutcome, asOf time.Time, opts Options) []StrategyEVResult` orchestrating grouping by `(strategy_id, regime)` → windows → slope → weight → rank. `StrategyEVResult` carries strategy_id, regime, ev_30d, ev_90d, ev_all_time, ev_slope (`*float64`), ev_weight, status, rank.
- `persist.go` — `Recompute(ctx, db *gorm.DB, repo TradeOutcomeRepository, asOf time.Time, opts Options) ([]StrategyEVResult, error)`: fetch → filter retired → `Compute` → write one `tradingstack.StrategyEvWeight` row per result. Retired ids are dropped before compute so they yield no row.
- `errors.go` — sentinel errors (e.g. `ErrNoTrades` handling is non-fatal; empty input yields zero results).
- `options.go` — `Options{ BucketDays int (default 30); Retired map[string]bool }`.
- `ev_test.go`, `slope_test.go`, `weights_test.go` — pure unit tests with hand-computed fixtures.
- `evtracker_test.go` — testcontainers Postgres persistence tests (migrate via `tradingstack.MigrateTradingStack`, run `Recompute` with the in-memory repo, assert rows).

## Data flow

`TradeOutcomeRepository.FetchTradeOutcomes(asOf)` → `[]TradeOutcome` → drop retired → group by `(strategy_id, regime)` → for each group compute ev_30d / ev_90d / ev_all_time (window filter + `ComputeEV`) and slope (bucket + OLS) → `ClassifyWeight(slope)` → assemble `[]StrategyEVResult` → `RankByEV` → write each result as a `strategy_ev_weights` row with `computed_at = asOf`. Downstream (the scanner optimizer, a future recommendation engine) reads `strategy_ev_weights`; this change only writes it.

## Out of scope

- **Net-of-cost EV.** The formula here is gross. Subtracting commissions/spread/borrow to produce `net_ev` is owned by `net-ev-cost-model`; this change does not read or write cost fields.
- **Live trade source.** The concrete `TradeOutcomeRepository` backed by real closed-trade data (and any table that stores live trades) accrues later. Overnight ships only the interface + the in-memory fixture implementation.
- **Recompute scheduling / operator CLI.** No cron, no long-running worker, no `cmd/` binary wired to a live DB. The only runnable surface added is the `test:ev-tracker` task target; a production recompute entry point is a future change.
- **Auto-retirement.** Deciding that a strategy should be retired (e.g. "decaying 3 consecutive regimes") is recommendation-engine territory. Here retirement is an explicit input set; the engine only excludes those ids.
- **Regime classification.** Regime tags arrive on the trade inputs; classifying regimes is not part of this change.

## Dependency ordering on other batch changes

Depends on `trading-stack-schema` — this change imports `tradingstack.StrategyEvWeight` and calls `tradingstack.MigrateTradingStack` in its tests, so it must be authored after that change merges. It shares no files with `net-ev-cost-model`, `fidelity-checker`, `crowding-detection`, or the other Wave-3 changes and can be developed in parallel with them once the schema lands. No dependency on the Phase-2 refactor track.

## Verification gates

- **Unit tests (hand-computed fixtures)** — EV formula, breakeven exclusion, window boundaries, OLS slope on a known linear series, every weight band including the exact 0.1 / 0.0 boundaries, null-slope handling, and ranking order. Run via `task test:ev-tracker`.
- **Testcontainers persistence tests** — `Recompute` against a real Postgres: correct row count, per-field equality with computed values, null slope → SQL NULL, retired strategy → no row, re-run reflects changed inputs. Run via `task test:ev-tracker`.
- **G1** — `go build ./src/go/... ./cmd/...` green (new package compiles, nothing else broken).
- **G2** — `task test` green (existing backtester-api suite unaffected).
- **G6** — Fable adversarial review of the diff before commit: formula faithfulness, band-boundary correctness, determinism (no wall-clock/random reads), no leakage into playground or cost logic.

## Deferred validation

None. All gates for this Tier A change run fully overnight against local unit tests and a testcontainers Postgres. The live-trade feed is *out of scope* (a future change), not deferred validation of this change — everything shipped here is validated by fixtures tonight.
