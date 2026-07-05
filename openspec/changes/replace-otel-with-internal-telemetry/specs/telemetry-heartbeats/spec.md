## ADDED Requirements

### Requirement: Heartbeat ingestion endpoint
The trading server SHALL expose `POST /telemetry/heartbeat` on the existing REST mux (:8080) accepting a JSON payload identifying the source (kind: strategy or datasource, name, and metadata such as playground id, symbol, state, tick/check count). A valid heartbeat SHALL update the in-memory registry's last-seen state and upsert the source's `telemetry_heartbeats` row; an invalid payload SHALL be rejected with a 4xx status without side effects.

#### Scenario: Strategy heartbeat is recorded
- **WHEN** a client posts a valid strategy heartbeat
- **THEN** the server responds 2xx, the registry's last-seen for that source is updated, and the heartbeat row is upserted

#### Scenario: Malformed heartbeat is rejected
- **WHEN** a client posts a payload missing the source kind or name
- **THEN** the server responds 4xx and records nothing

### Requirement: Python clients report heartbeats to the server
`StrategyHeartbeat` and `DatasourceHeartbeat` SHALL keep their existing public API (`start`, `record_tick`/`record_check`, `set_state`, `stop`) but their daemon threads SHALL report via the heartbeat endpoint every 30 seconds instead of recording OpenTelemetry gauges. The Python client modules SHALL NOT import any `opentelemetry` package.

#### Scenario: Running strategy beats every interval
- **WHEN** a strategy with a started heartbeat runs for multiple intervals
- **THEN** the server receives a heartbeat per interval carrying the strategy's name, state, and tick count

#### Scenario: No OTel imports in Python clients
- **WHEN** `src/clients/python` is grepped for `opentelemetry` after the change
- **THEN** there are no matches

### Requirement: Telemetry delivery failures never disturb the strategy
A failed heartbeat post (connection refused, timeout, non-2xx) SHALL log a warning and leave the strategy's trading behavior unaffected; the heartbeat thread SHALL continue attempting subsequent beats.

#### Scenario: Server briefly unreachable
- **WHEN** the heartbeat endpoint is down for one interval and back for the next
- **THEN** the strategy keeps trading, one warning is logged, and the next beat is delivered normally

### Requirement: Staleness is tracked per source
The server SHALL consider a heartbeat source stale when its last-seen timestamp is older than the staleness threshold (default 90 seconds — three missed beats — env-overridable via `TELEMETRY_HEARTBEAT_STALE_AFTER`), and SHALL expose per-source staleness to the alerting rules and the status query. The server SHALL NOT heartbeat to itself.

#### Scenario: Silent strategy becomes stale
- **WHEN** a previously-beating strategy sends nothing for longer than the threshold
- **THEN** the server marks that source stale while other sources remain fresh
