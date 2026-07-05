# Tasks — feed-health-staleness

## 1. Package scaffold

- [x] 1.1 Create `src/go/feedhealth/` package with `errors.go` (`ErrInvalidThresholdOrdering`, `ErrConfigNotFound`).
- [x] 1.2 `clock.go` — `Clock` interface, `RealClock`, exported `FakeClock` (settable + `Advance(d time.Duration)`).
- [x] 1.3 `status.go` — `FeedHealthStatus` type with `Healthy`/`Degraded`/`Stale` constants and `String()`.

## 2. Threshold config

- [x] 2.1 `config.go` — `ThresholdConfig` type + `LoadThresholdConfig(path string)`, validating `staleness_threshold > expected_interval` per asset class, returning `ErrInvalidThresholdOrdering` on violation.
- [x] 2.2 Path resolution helper: `FEED_HEALTH_CONFIG_PATH` env var, else `${TRADING_PROJECT_DIR}/src/go/feed-health-config.yaml`.
- [x] 2.3 Add `src/go/feed-health-config.yaml` with `equity` and `option` asset classes seeded with sane defaults (equity: 15s/60s; option: 30s/120s).
- [x] 2.4 `config_test.go` — load valid config with two asset classes, no cross-contamination; invalid ordering rejected.

## 3. Heartbeat monitor

- [x] 3.1 `heartbeat.go` — `HeartbeatMonitor` with mutex-guarded per-asset-class last-observed map; `Observe`, `LastObserved`.
- [x] 3.2 `Status(assetClass)` — computes `healthy`/`degraded`/`stale` from `clock.Now() - LastObserved` vs. thresholds; never-observed asset class reports `stale`.
- [x] 3.3 `ShouldPauseScanning(assetClass)` — `true` iff `Status == stale`; re-evaluates fresh on every call (no latch).
- [x] 3.4 `heartbeat_test.go` — all spec scenarios with `FakeClock`: per-asset-class isolation, replacement on later observation, healthy/degraded/stale boundary transitions, never-observed defaults stale, auto-pause true/false, auto-recovery on fresh `Observe` without a reset call.

## 4. scan_results tagging

- [x] 4.1 Add nullable `FeedHealth *string` field + `feed_health` column to `tradingstack.ScanResult` (in `src/go/tradingstack/scan_result.go`) via an additive, idempotent migration statement — do not alter any existing column, tag, or constraint on that model.
- [x] 4.2 `scan_tag.go` — `TagScanResult(result *tradingstack.ScanResult, status FeedHealthStatus)` setting `FeedHealth` to the status's string form.
- [x] 4.3 `scan_tag_test.go` — tagging sets `FeedHealth` to `"healthy"`/`"degraded"`/`"stale"` correctly; migration test confirms pre-existing `scan_results` columns/constraints are untouched after the new column is added.

## 5. Taskfile wiring

- [x] 5.1 Add `test:feed-health` target to `taskfile.yml` running `go test -count=1 ./src/go/feedhealth/...`, exiting non-zero on failure.

## 6. Verification and closeout

- [x] 6.1 Unit tests with fake clocks pass: `task test:feed-health` green, covering every scenario in `specs/feed-health-staleness/spec.md`.
- [x] 6.2 G1 — `go build ./src/go/... ./cmd/...` green.
- [x] 6.3 G2 — `task test` green (existing backtester-api suite unaffected).
- [ ] 6.4 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time).
