# Design — feed-health-staleness

## Approach

A small, self-contained Go package that turns "is the Feed still ticking?" into a testable state machine: a `HeartbeatMonitor` records the last-observed tick timestamp per asset class, and derives a three-state `FeedHealthStatus` (`healthy` / `degraded` / `stale`) purely from elapsed time versus two configured thresholds. Time is supplied through an injectable `Clock`, so every scenario (healthy → degraded → stale → recovered) is driven with a `FakeClock` in unit tests — no wall-clock sleeps, no flaky timing. The monitor exposes two read paths: `ShouldPauseScanning` (a hard gate a future scanner calls before running a scan cycle) and a tagging helper that stamps the computed status onto `scan_results` rows (a soft signal for downstream filtering/downweighting). Nothing in this change talks to the real Feed (Massive/Polygon) yet — it consumes ticks pushed to it via `Observe`; wiring an actual tick source to call `Observe` is left to whichever component owns tick ingestion for real-time modes (out of scope here, see below).

## Package and file layout

New package `src/go/feedhealth/` (import path `github.com/jiaming2012/slack-trading/src/go/feedhealth`):

- `clock.go` — `Clock` interface (`Now() time.Time`); `RealClock` (wraps `time.Now()`); `FakeClock` (settable/advanceable, exported for reuse by later packages' tests, e.g. `scanner-l1-l2`).
- `status.go` — `FeedHealthStatus` type (`Healthy`, `Degraded`, `Stale` constants) + `String()`.
- `heartbeat.go` — `HeartbeatMonitor` struct: `clock Clock`, `thresholds ThresholdConfig`, a mutex-guarded `map[string]time.Time` of last-observed-per-asset-class. Methods: `Observe(assetClass string, at time.Time)`, `LastObserved(assetClass string) (time.Time, bool)`, `Status(assetClass string) FeedHealthStatus`, `ShouldPauseScanning(assetClass string) bool`.
- `config.go` — `ThresholdConfig` (map of asset class → `{ExpectedInterval, StalenessThreshold time.Duration}`) + `LoadThresholdConfig(path string) (ThresholdConfig, error)`, validating `staleness_threshold > expected_interval` per asset class; path resolution helper reading `FEED_HEALTH_CONFIG_PATH` env var with fallback to `${TRADING_PROJECT_DIR}/src/go/feed-health-config.yaml` (mirrors `OPTIONS_CONFIG_PATH` resolution already used for options config).
- `scan_tag.go` — `TagScanResult(result *tradingstack.ScanResult, status FeedHealthStatus)` setting `result.FeedHealth`; imports the `tradingstack` package introduced by `trading-stack-schema`.
- `errors.go` — sentinel errors: `ErrInvalidThresholdOrdering`, `ErrConfigNotFound`.
- `clock_test.go`, `heartbeat_test.go`, `config_test.go`, `scan_tag_test.go` — unit tests, all driven by `FakeClock`.

New config file: `src/go/feed-health-config.yaml` (repo-relative, sibling to `options-config.yaml`):

```yaml
asset_classes:
  equity:
    expected_interval: 15s
    staleness_threshold: 60s
  option:
    expected_interval: 30s
    staleness_threshold: 120s
```

## Data flow

1. Whatever component observes real ticks for a given `Mode` (out of scope for this change — see below) calls `HeartbeatMonitor.Observe(assetClass, clock.Now())` each time a new tick/candle arrives for that asset class.
2. Before running a scan cycle (future `scanner-l1-l2` integration point, not built here), the caller checks `ShouldPauseScanning(assetClass)`; `true` means skip the cycle for that asset class rather than scan on stale data.
3. When a scan cycle does run, the caller calls `Status(assetClass)` and applies `TagScanResult` to each `tradingstack.ScanResult` it builds, before persisting it — giving downstream EV/optimizer components a `feed_health` column to filter or downweight on.
4. Recovery is automatic and un-latched: the moment a fresh `Observe` lands, the next `Status`/`ShouldPauseScanning` call reflects it. There is no separate "resume" call or operator acknowledgment step (unlike the kill-switch cooldown protocol in Finding 2, which is intentionally a different, human-gated mechanism for a different failure class).

## Out of scope

- **Secondary-source cross-check** (comparing last price/volume against a second, e.g. free-delayed, data provider) — explicitly deferred per Finding 9's own cost/benefit call: "Heartbeat monitoring alone captures most failure modes." No second provider integration, no new external dependency.
- **Wiring a real tick source to call `Observe`.** This change builds the monitor and its interface only. Which component calls `Observe` for real-time Paper/Margin ticks is a decision for whichever change wires up live tick ingestion (not yet drafted); Simulation mode (historical replay) is not a target for this layer at all — replayed candles are not subject to "is the Feed alive right now" semantics by construction.
- **Wiring `ShouldPauseScanning` into an actual scanner.** `scanner-l1-l2` does not exist yet in this batch's build order; this change ships the gate function, not the caller.
- **Alerting/paging integration** (Grafana/Loki alert rules referenced in the roadmap's `verify-grafana-heartbeat-and-stale-alert` card) — that card is explicitly out of scope for tonight's run (operator/Windows-desktop ritual) and is a separate, already-tracked roadmap item.
- **Any change to existing playground tables, models, migrations, or the live/paper broker order path.**

## Dependency ordering within the batch

This change depends on `trading-stack-schema` for the `tradingstack.ScanResult` model that `scan_tag.go` imports and extends with a `feed_health` column. It must be authored and merged after `trading-stack-schema` lands (per the run plan's Phase 3a: `trading-stack-schema` lands first; `feed-health-staleness` is authored alongside `fidelity-checker`/`ev-tracker` with its interfaces stubbed against that change's models). The heartbeat/clock/threshold/auto-pause requirements have no dependency on `trading-stack-schema` and could be built, tested, and merged independently if the schema change were delayed — only the `feed_health` tagging requirement and its migration statement are blocked on it. If `trading-stack-schema` fails or is reverted, this change should still land with everything except the tagging requirement, noted as partial in the morning report.

## Verification gates

- **Unit tests with fake clocks** — every scenario in `specs/feed-health-staleness/spec.md` (healthy/degraded/stale transitions, per-asset-class isolation, auto-pause and auto-recovery, threshold load/validation, tagging) is exercised with `FakeClock`, zero wall-clock sleeps, deterministic and fast.
- **G1** — `go build ./src/go/... ./cmd/...` green (new package compiles, nothing else broken).
- **G2** — `task test` green (existing backtester-api suite unaffected; this package lives outside that module's test scope, exercised separately by `test:feed-health`).
- **`task test:feed-health`** — the new package's own test target, run in addition to G1/G2, exits non-zero on any failure.

## Deferred validation

A **live feed outage drill** — actually starving the real Massive/Polygon feed (or simulating an outage against a live connection) to confirm the heartbeat monitor detects it, staleness escalates through `degraded`→`stale`, and a real scanner would in fact pause — is deferred. It requires a live Paper/Margin-mode feed connection and a running scanner, neither of which is safe or available to exercise overnight (hard prohibition on live/paper broker activity; `scanner-l1-l2` isn't built yet in this batch). This drill should be scheduled as a manual follow-up once `scanner-l1-l2` lands and a Paper-mode session is available to interrupt deliberately.
