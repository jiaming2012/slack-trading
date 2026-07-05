# feed-health-staleness

## ADDED Requirements

### Requirement: Injectable clock for deterministic staleness tests

The system SHALL provide a `Clock` interface (`Now() time.Time`) in the `feedhealth` package with two implementations: `RealClock` (delegates to `time.Now()`) and `FakeClock` (holds a settable, advanceable time value). Every component that computes elapsed time for staleness detection SHALL accept a `Clock` rather than calling `time.Now()` directly, so tests can advance time deterministically without wall-clock sleeps.

#### Scenario: FakeClock reports the time it was set to

- **WHEN** a `FakeClock` is created with an initial time `T0` and no further mutation occurs
- **THEN** calling `Now()` on it returns exactly `T0`

#### Scenario: Advancing a FakeClock changes subsequent Now() calls

- **WHEN** a `FakeClock` is set to `T0` and then advanced by a duration `D`
- **THEN** calling `Now()` returns `T0 + D`

### Requirement: Per-asset-class heartbeat tracking

The system SHALL provide a `HeartbeatMonitor` that records the last-observed tick timestamp independently per asset class (e.g. `equity`, `option`), via an `Observe(assetClass string, at time.Time)` method. Asset classes not yet observed SHALL be treated as unknown/never-ticked rather than implicitly healthy.

#### Scenario: Observing a tick records it for that asset class only

- **WHEN** `Observe("equity", t1)` is called and no observation has been made for `"option"`
- **THEN** `LastObserved("equity")` returns `t1`
- **AND** `LastObserved("option")` reports no observation has occurred

#### Scenario: A later observation replaces the recorded timestamp

- **WHEN** `Observe("equity", t1)` is called and then `Observe("equity", t2)` is called with `t2` after `t1`
- **THEN** `LastObserved("equity")` returns `t2`

### Requirement: Configurable per-asset-class thresholds

The system SHALL load, per asset class, an `expected_interval` (the maximum gap between ticks considered healthy) and a `staleness_threshold` (the gap beyond which the Feed is considered stale for that asset class, strictly greater than `expected_interval`) from a YAML config file. The config file path SHALL be resolved from a `FEED_HEALTH_CONFIG_PATH` env var if set, else from `${TRADING_PROJECT_DIR}/src/go/feed-health-config.yaml`, mirroring the existing `OPTIONS_CONFIG_PATH` / `OPTIONS_CONFIG_FILE` convention. Loading SHALL fail with a descriptive error if `staleness_threshold <= expected_interval` for any configured asset class.

#### Scenario: Thresholds load per asset class from YAML

- **WHEN** a config file declares `expected_interval` and `staleness_threshold` for `equity` and separately for `option`
- **THEN** the loaded config returns the `equity` thresholds when queried for `"equity"` and the `option` thresholds when queried for `"option"`, without cross-contamination

#### Scenario: Invalid threshold ordering is rejected at load time

- **WHEN** a config file declares an asset class whose `staleness_threshold` is less than or equal to its `expected_interval`
- **THEN** loading the config returns a non-nil error and no threshold set is usable

### Requirement: Three-state feed health status derived from elapsed time

The system SHALL compute a `FeedHealthStatus` (`healthy`, `degraded`, or `stale`) for an asset class from the elapsed time between the monitor's `Clock.Now()` and that asset class's `LastObserved` timestamp, compared against its configured thresholds: elapsed `<= expected_interval` is `healthy`; elapsed `> expected_interval` and `<= staleness_threshold` is `degraded`; elapsed `> staleness_threshold` is `stale`. An asset class with no observation yet SHALL report `stale`.

#### Scenario: Elapsed time within expected_interval is healthy

- **WHEN** a tick is observed for `equity` at `t0`, thresholds are `expected_interval=15s`/`staleness_threshold=60s`, and the fake clock is advanced to `t0+10s`
- **THEN** `Status("equity")` returns `healthy`

#### Scenario: Elapsed time between the two thresholds is degraded

- **WHEN** a tick is observed for `equity` at `t0` with the same thresholds and the fake clock is advanced to `t0+30s`
- **THEN** `Status("equity")` returns `degraded`

#### Scenario: Elapsed time beyond staleness_threshold is stale

- **WHEN** a tick is observed for `equity` at `t0` with the same thresholds and the fake clock is advanced to `t0+90s`
- **THEN** `Status("equity")` returns `stale`

#### Scenario: A never-observed asset class is stale

- **WHEN** `Status("option")` is queried and no `Observe` call has ever been made for `"option"`
- **THEN** the returned status is `stale`

### Requirement: Auto-pause scanning on stale status, with automatic resume

The system SHALL expose `ShouldPauseScanning(assetClass string) bool` that returns `true` when that asset class's computed `FeedHealthStatus` is `stale`, and `false` for `healthy` or `degraded`. Once a fresh tick is observed for a previously stale asset class, `ShouldPauseScanning` SHALL re-evaluate to `false` on the next call without requiring a manual reset — recovery is automatic, not latched.

#### Scenario: Scanning is blocked while an asset class is stale

- **WHEN** an asset class's `FeedHealthStatus` is `stale`
- **THEN** `ShouldPauseScanning` for that asset class returns `true`

#### Scenario: Scanning is not blocked while degraded

- **WHEN** an asset class's `FeedHealthStatus` is `degraded`
- **THEN** `ShouldPauseScanning` for that asset class returns `false`

#### Scenario: Auto-pause clears automatically once a fresh tick arrives

- **WHEN** an asset class is `stale`, `ShouldPauseScanning` returns `true`, and then a fresh `Observe` call is made at the current clock time
- **THEN** the next call to `ShouldPauseScanning` for that asset class returns `false` without any explicit acknowledgment or reset call

### Requirement: feed_health tagging on scan results

The system SHALL provide a helper that sets a `FeedHealth` field on a `tradingstack.ScanResult` value to the string form of the computed `FeedHealthStatus` (`"healthy"`, `"degraded"`, or `"stale"`) for the scan result's asset class at the time the tag is applied, via an additive nullable `feed_health` column added to the existing `scan_results` table by an idempotent migration statement. This requirement depends on the `trading-stack-schema` change having landed; it does not alter any existing column, constraint, or table that change's spec declares.

#### Scenario: A scan result is tagged healthy when the Feed is healthy

- **WHEN** the tagging helper is applied to a `ScanResult` for an asset class whose computed status is `healthy`
- **THEN** the `ScanResult.FeedHealth` field equals `"healthy"`

#### Scenario: A scan result is tagged degraded when the Feed is degraded

- **WHEN** the tagging helper is applied to a `ScanResult` for an asset class whose computed status is `degraded`
- **THEN** the `ScanResult.FeedHealth` field equals `"degraded"`

#### Scenario: The additive migration does not alter existing scan_results columns

- **WHEN** the `feed_health` column migration runs against a database that already has the `scan_results` table from `trading-stack-schema`
- **THEN** the table retains every pre-existing column and constraint unchanged, and gains a nullable `feed_health` column

### Requirement: Independent test task for the feed health package

The system SHALL provide a `task test:feed-health` Taskfile target that runs the `feedhealth` package's unit tests in isolation from the full backtester-api suite. The target SHALL exit non-zero if any test fails.

#### Scenario: Task target runs the feed health unit test suite

- **WHEN** an operator runs `task test:feed-health`
- **THEN** the `feedhealth` package's unit tests execute and the command exits zero when all tests pass, non-zero otherwise
