## ADDED Requirements

### Requirement: Metric snapshots persist to the playground database
A snapshot writer goroutine SHALL periodically (default every 60 seconds, env-overridable via `TELEMETRY_SNAPSHOT_INTERVAL`) write every registry series to a `telemetry_metrics` table in the existing playground Postgres database, recording timestamp, metric name, labels, kind, and value. The table SHALL be created by the server's existing auto-migration at startup.

#### Scenario: Snapshot rows appear after an interval
- **WHEN** the server has been running with the writer started and at least one snapshot interval has elapsed
- **THEN** `telemetry_metrics` contains rows for the current registry series with a recent timestamp

#### Scenario: Counters persist cumulatively
- **WHEN** a counter is incremented across two snapshot intervals
- **THEN** consecutive rows for that series hold non-decreasing cumulative values (rates are derivable by SQL window over snapshots)

#### Scenario: Fresh database boots cleanly
- **WHEN** the server starts against a database without telemetry tables
- **THEN** auto-migration creates them and the first snapshot succeeds

### Requirement: Heartbeats persist on arrival
Heartbeat state SHALL be written to a `telemetry_heartbeats` table as an upsert at ingest time (one row per source), recording source kind, name, metadata, last-seen timestamp, and beat count — a heartbeat's durability SHALL NOT wait for a snapshot tick.

#### Scenario: Heartbeat row updates on each beat
- **WHEN** a strategy heartbeat arrives twice
- **THEN** its single `telemetry_heartbeats` row shows the later last-seen timestamp and an incremented beat count

### Requirement: 30-day retention prune
A prune goroutine SHALL delete `telemetry_metrics` rows older than 30 days, running at least daily, bounding the operational time-series data carried by the trading database per ADR-0005.

#### Scenario: Old rows are removed
- **WHEN** the prune runs against rows older than 30 days
- **THEN** those rows are deleted and younger rows are untouched

### Requirement: Persistence failures never disturb trading
Snapshot, upsert, or prune failures SHALL be logged and retried on the next cycle; they SHALL NOT propagate errors into order processing, tick handling, or client requests.

#### Scenario: Database outage during a snapshot
- **WHEN** the database is unreachable at a snapshot tick
- **THEN** the writer logs the failure, trading continues, and the next successful tick persists current registry state
