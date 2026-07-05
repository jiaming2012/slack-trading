# Feed Health: Heartbeat & Staleness Detection

## Why

Every downstream v4 component (scanner, simulator, optimizers) silently assumes the market-data Feed is fresh; today nothing detects a stalled, delayed, or degraded Feed, so an outage can drive scans — and eventually trades — on stale data with no observable signal that anything is wrong.

## What Changes

- Add a heartbeat monitor that tracks the last-observed tick timestamp per asset class, driven by an injectable `Clock` so staleness detection is deterministic and testable with fake clocks (no wall-clock sleeps in tests).
- Add two configurable thresholds per asset class: an `expected_interval` (healthy boundary) and a `staleness_threshold` (auto-pause boundary), loaded from a new YAML config file via an env-var path, mirroring the existing `OPTIONS_CONFIG_PATH` pattern.
- Compute a three-state `feed_health` status per asset class — `healthy`, `degraded`, `stale` — from elapsed time since the last observed tick relative to those thresholds.
- Auto-pause scanning for an asset class once its Feed staleness crosses the `staleness_threshold`, rather than letting a scan run against stale data; scanning is allowed to resume automatically once a fresh tick is observed for that asset class.
- Tag scan results with the computed `feed_health` status at scan time via an additive, nullable column on the existing `tradingstack.ScanResult` model (owned by `trading-stack-schema`), so downstream components can filter or downweight scans from degraded-feed periods. **Requires `trading-stack-schema` to have landed first** — no table or constraint declared by that change's spec is altered.
- Add a dedicated Taskfile target (`task test:feed-health`) so the new package's unit tests can run independently of the full backtester-api suite.
- Secondary-source cross-check against a second data provider is explicitly **OUT of scope** — the source finding calls it optional and cost-adding; heartbeat + staleness alone covers most failure modes.
- No changes to any existing playground table, model, migration path, or the live/paper broker order path. **Not BREAKING.**

## Capabilities

### New Capabilities

- `feed-health-staleness`

### Modified Capabilities

(none)

## Impact

- New package: `src/go/feedhealth/` — clock abstraction (`Clock`, `RealClock`, `FakeClock`), heartbeat monitor, `FeedHealthStatus` enum, threshold config loader, and a `scan_results` tagging helper. No existing package's logic is modified.
- One additive touch point outside the new package: a new nullable `feed_health` column + idempotent migration statement on `tradingstack.ScanResult` (the model introduced by `trading-stack-schema`). This extends that model without altering any field, constraint, or table its own spec declares.
- New config file `src/go/feed-health-config.yaml` and a new `FEED_HEALTH_CONFIG_PATH` env var, following the existing `OPTIONS_CONFIG_PATH` / `OPTIONS_CONFIG_FILE` convention.
- New Taskfile target: `test:feed-health`.
- **Depends on `trading-stack-schema`** — must land and archive first, since this change extends the `tradingstack.ScanResult` model it introduces. If `trading-stack-schema` fails or is reverted, this change's tagging requirement cannot be implemented (heartbeat/staleness/auto-pause requirements are otherwise independent).
- Consumed later by `scanner-l1-l2` (would call `ShouldPauseScanning` before a scan cycle) and by EV/optimizer components (would read the `feed_health` tag to downweight degraded periods) — those integrations are out of scope for this change, which only builds the layer and its interface.
- Verification is local-only: fake-clock unit tests, `go build`, and the existing `task test` suite. No live feed connection, no prod DB, no live/paper broker orders. A live feed outage drill is explicitly deferred (see `design.md`).
