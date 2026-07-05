# Tasks — crowding-detection

## 1. Package scaffolding

- [x] 1.1 Create `src/go/tradingstack/crowding/` package skeleton (package `crowding`), importing `github.com/jiaming2012/slack-trading/src/go/tradingstack` for reference types.
- [x] 1.2 Add `candidate.go` with the `ScanCandidate` struct (`ScanResultID`, `Ticker`, `ScannedAt`, `StrategyIDs []string`).

## 2. Overlap detection core logic

- [x] 2.1 Add `detector.go` with the `CrowdingResult` struct (`ScannedAt`, `TotalCandidates`, `OverlappingCandidates`, `OverlapPct`, `ThresholdPct`, `Flagged`, `OverlappingCandidateDetails []ScanCandidate`).
- [x] 2.2 Implement `DetectCrowding(candidates []ScanCandidate, thresholdPct float64) (CrowdingResult, error)`: count total candidates, count candidates with `len(distinct StrategyIDs) >= 2`, compute `OverlapPct`, set `Flagged = OverlapPct > thresholdPct` (strict).
- [x] 2.3 Add `config.go` with `DefaultOverlapThresholdPct = 30.0` and an optional `CROWDING_OVERLAP_THRESHOLD_PCT` env override via `utils.GetEnv`.

## 3. Persistence models and migration

- [x] 3.1 Add `models.go` with `CrowdingMetric` (`TableName() "crowding_metrics"`) and `CrowdingFlaggedCandidate` (`TableName() "crowding_flagged_candidates"`), plus a shared `BeforeCreate` UUID hook.
- [x] 3.2 Add `migrate.go` with `MigrateCrowdingDetection(db *gorm.DB) error`: `AutoMigrate` both models and add the `crowding_metric_id` / `scan_result_id` foreign keys idempotently, touching no other table.

## 4. Store abstraction

- [x] 4.1 Add `store.go` with the `CrowdingStore` interface (`Persist(metric CrowdingMetric, flagged []CrowdingFlaggedCandidate) error`).
- [x] 4.2 Implement `GormCrowdingStore` (production, writes `CrowdingMetric` + `CrowdingFlaggedCandidate` rows in one transaction).
- [x] 4.3 Implement `FakeCrowdingStore` (in-memory, records the last-persisted metric and flagged candidates, used only by tests).

## 5. Fixture-based unit tests

- [x] 5.1 Write `crowding_test.go` fixture: 10 candidates, exactly 3 overlapping (>=2 distinct strategies) — assert `TotalCandidates=10`, `OverlappingCandidates=3`, `OverlapPct=30.0`.
- [x] 5.2 Write fixture asserting `OverlapPct=0.0` when no candidate has more than one strategy.
- [x] 5.3 Write fixture asserting `OverlapPct=100.0` when every candidate has >=2 strategies.
- [x] 5.4 Write fixture with candidates that have zero simulating strategies — assert they count toward `TotalCandidates` but never `OverlappingCandidates`.
- [x] 5.5 Write threshold-boundary tests: overlap above threshold flags true; overlap exactly at threshold flags false; overlap below threshold flags false.
- [x] 5.6 Write `FakeCrowdingStore` test asserting `Persist` records the same metric values and flagged-candidate count that were passed in.

## 6. Taskfile wiring

- [x] 6.1 Add `test:crowding-detection` target to `taskfile.yml` running `go test -count=1 ./src/go/tradingstack/crowding/...` with `TRADING_PROJECT_DIR` set, matching the style of the existing `test` and `test:trading-stack` targets.

## 7. Verification and closeout

- [x] 7.1 Run unit tests with fixture overlap scenarios (`task test:crowding-detection`) — all pass, including the known-percentage, zero-overlap, full-overlap, no-strategy, and threshold-boundary cases.
- [x] 7.2 Run G1: `go build ./src/go/... ./cmd/...` — green.
- [x] 7.3 Run G2: `task test` — green (existing `backtester-api` suite unaffected).
- [x] 7.4 Run `openspec validate crowding-detection --strict` — passes.
- [ ] 7.5 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time).
