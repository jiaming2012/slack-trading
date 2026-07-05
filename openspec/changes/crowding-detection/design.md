# Design — crowding-detection

## Approach

Treat crowding detection as a pure aggregation over data already produced by the v4 trading-stack schema (`scan_results`, `sim_outcomes`), plus a thin persistence layer for the resulting metrics. The core algorithm (`DetectCrowding`) takes no database dependency at all — it operates on an in-memory slice of `ScanCandidate` fixtures — so the deterministic overlap math can be exhaustively unit tested with hand-computed percentages, exactly as the brief requires ("unit tests w/ fixture overlap scenarios"). A thin repository step (out of scope for this change's tests) is responsible for turning a real `scanned_at` cycle's rows from `scan_results`/`sim_outcomes` into that same `ScanCandidate` shape before calling the pure function.

**Definition of "scan cycle"**: all `scan_results` rows sharing one exact `scanned_at` timestamp, per the architecture doc's description of the scanner writing "a full feature vector to `scan_results` at scan time" for the whole batch. This is an assumption this change makes explicit; if a future change introduces an explicit batch/cycle identifier, the cycle-grouping key can be swapped without touching the aggregation math.

**Definition of "overlap"**: a candidate (one `scan_results` row) is "overlapping" when two or more *distinct* `sim_outcomes.strategy_id` values reference it via `scan_result_id`. This directly operationalizes Finding 11 ("multiple strategies converge on the same trades") using the schema `trading-stack-schema` already defines, without requiring any new column on `scan_results` or `sim_outcomes`.

## Package and file layout

New package `src/go/tradingstack/crowding/` (import path `github.com/jiaming2012/slack-trading/src/go/tradingstack/crowding`):

- `candidate.go` — `ScanCandidate` struct (the input shape: `ScanResultID`, `Ticker`, `ScannedAt`, `StrategyIDs []string` — the distinct strategies with a `sim_outcomes` row for this candidate).
- `detector.go` — `DetectCrowding(candidates []ScanCandidate, thresholdPct float64) (CrowdingResult, error)` — pure function, no I/O. `CrowdingResult` holds `ScannedAt`, `TotalCandidates`, `OverlappingCandidates`, `OverlapPct`, `ThresholdPct`, `Flagged`, and the list of overlapping candidates (for building `CrowdingFlaggedCandidate` rows).
- `models.go` — `CrowdingMetric` (`TableName() "crowding_metrics"`) and `CrowdingFlaggedCandidate` (`TableName() "crowding_flagged_candidates"`) GORM models, plus a shared `BeforeCreate` UUID hook mirroring the pattern already established in `tradingstack/ids.go`.
- `migrate.go` — `MigrateCrowdingDetection(db *gorm.DB) error`: `AutoMigrate` the two models plus the `crowding_flagged_candidates.crowding_metric_id` and `.scan_result_id` foreign keys.
- `store.go` — `CrowdingStore` interface (`Persist(metric CrowdingMetric, flagged []CrowdingFlaggedCandidate) error`), `GormCrowdingStore` (production, writes via GORM in a transaction), `FakeCrowdingStore` (in-memory, used only by tests).
- `config.go` — `DefaultOverlapThresholdPct = 30.0`; threshold is a plain function parameter / config value, not a new entry in `options-config.yaml` (that file is scoped to options trading, not the v4 stack) — an env var `CROWDING_OVERLAP_THRESHOLD_PCT` may override the default, read via the existing `utils.GetEnv` helper.
- `crowding_test.go` — fixture-driven unit tests for `DetectCrowding` (known percentages, zero overlap, full overlap, no-simulating-strategy candidates, threshold boundary) and for `FakeCrowdingStore`.

## Data flow

1. (Out of scope for this change) Something — a future scheduler or the scanner itself — reads the `scan_results` and `sim_outcomes` rows for a completed `scanned_at` cycle and builds `[]ScanCandidate`.
2. `DetectCrowding` computes `CrowdingResult` from that slice and the configured threshold — pure, deterministic, no I/O.
3. The caller builds a `CrowdingMetric` and `[]CrowdingFlaggedCandidate` from the result and calls `CrowdingStore.Persist`.
4. `portfolio-risk-overlay` (a separate, not-yet-drafted change) is the intended reader of `crowding_metrics` / `crowding_flagged_candidates` for pre-trade concentration checks. This change does not read from or write to anything in that (nonexistent) change — it only leaves data in a shape a future change can query.

## Out of scope

- Building the repository/query step that turns real `scan_results` + `sim_outcomes` rows into `[]ScanCandidate` against a live database. Only the pure aggregation function and an in-memory-fake-backed store are unit tested this change; the real query is a small, mechanical GORM join that can be added when a caller (scanner integration or `portfolio-risk-overlay`) actually needs it, to avoid speculative code with no consumer yet.
- Wiring crowding detection into the live/paper scanning pipeline (`scanner-l1-l2` does not exist yet either).
- Any consumption logic inside `portfolio-risk-overlay` — that change, when drafted, owns its own read path against these tables.
- Any change to the six `trading-stack-schema` tables or models.
- Alerting/notification on a flagged cycle (Slack, email, etc.) — this change only persists the flag; routing it to an operator is a future concern.
- Partitioning/retention for the two new tables (mirrors `db-partitioning-retention`'s scope for the six schema tables, but that change does not currently list these two tables — a future change can extend it if volume warrants).

## Dependency ordering within the batch

Depends on `trading-stack-schema` (must land and be archived first — this change imports `ScanResult` and `SimOutcome` from the `tradingstack` package). Does not depend on and is not depended on by any other batch change; `portfolio-risk-overlay` is a soft downstream consumer (reads these tables once it exists) but is not a build dependency of this change.

## Verification gates

- **Unit tests with fixture overlap scenarios** — `go test ./src/go/tradingstack/crowding/...`: known-percentage fixture, zero-overlap fixture, full-overlap fixture, candidates-with-no-strategies fixture, threshold-boundary (at/above/below) fixtures, and `FakeCrowdingStore` persistence-shape assertions.
- **G1** — `go build ./src/go/... ./cmd/...` green (new package compiles, nothing else broken).
- **G2** — `task test` green (existing `backtester-api` suite is untouched by this change; `task test`'s `./...` scope is rooted at `src/go/backtester-api` and does not include the new package, so this gate is a regression check, not a coverage check for the new code).

## Deferred validation

The `GormCrowdingStore` production implementation and `MigrateCrowdingDetection` are written to the same pattern already proven in `trading-stack-schema` (testcontainers round-trip), but this change does not add a testcontainers-based integration test for them — the brief's verification gate for this Tier A card is unit tests plus G1/G2 only, not a new Postgres integration suite. A follow-up change (or an addendum to this one) should add a testcontainers round-trip test for `MigrateCrowdingDetection` and `GormCrowdingStore.Persist` before this store is wired into any real pipeline; until then, treat the production GORM path as reviewed-but-not-integration-tested.
