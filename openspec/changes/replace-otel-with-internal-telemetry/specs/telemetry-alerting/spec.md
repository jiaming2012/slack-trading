## ADDED Requirements

### Requirement: Server-side alert rule evaluation
A server goroutine SHALL evaluate alert rules every 30 seconds over the in-memory registry (not the database, so evaluation survives database slowness). v1 rules: (1) stale heartbeat — any registered strategy or datasource past the staleness threshold; (2) error-rate spike — the logrus-hook error counter exceeds a threshold (default 10, `TELEMETRY_ERROR_THRESHOLD`) within a rolling window (default 5 minutes, `TELEMETRY_ERROR_WINDOW`).

#### Scenario: Stale heartbeat fires an alert
- **WHEN** a heartbeat source crosses the staleness threshold
- **THEN** the next evaluation cycle fires a stale-heartbeat alert identifying the source

#### Scenario: Error burst fires an alert
- **WHEN** more errors than the threshold are logged within the window
- **THEN** an error-rate alert fires naming the observed count and window

### Requirement: Error counting via logrus hook
A logrus hook SHALL increment an error counter in the registry for every log entry at error level or above, replacing log-pipeline-based alerting. Log storage SHALL NOT be implemented (explicit v1 non-goal per ADR-0005).

#### Scenario: Logged error increments the counter
- **WHEN** any component logs at error level
- **THEN** the registry's error counter increases by one

### Requirement: Alert lifecycle with re-notification until acknowledged
Alerts SHALL have a firing→(acknowledged)→resolved lifecycle persisted in a `telemetry_alerts` table. A firing alert SHALL post to Slack (via the existing Slack notifier) and re-post every re-notify interval (default 30 minutes, env-overridable via `TELEMETRY_ALERT_RENOTIFY_INTERVAL`) until it is acknowledged or the condition resolves. Each Slack message SHALL include the alert id and how to acknowledge it. Resolution SHALL close the alert with a resolution notification whether or not it was acknowledged. The alert row SHALL be written regardless of Slack delivery success.

#### Scenario: Unacked alert keeps re-notifying
- **WHEN** a source stays stale across two re-notify intervals with no acknowledgement
- **THEN** Slack receives the firing notification and two re-notifications, each carrying the alert id

#### Scenario: Resolution closes an unacked alert
- **WHEN** a stale source beats again before anyone acknowledges the alert
- **THEN** a resolution notification is sent, the row records the resolved time, and re-notification stops

### Requirement: Slack delivery is retried until it succeeds
A failed Slack post SHALL NOT be dropped: the evaluation loop SHALL retry delivery on each subsequent cycle until a post succeeds, then resume the normal re-notify interval. Delivery failures SHALL be logged and SHALL NOT block rule evaluation; the persisted alert row SHALL exist independently of delivery.

#### Scenario: Slack outage at firing time
- **WHEN** Slack is unreachable when an alert fires and comes back two cycles later
- **THEN** the alert row is written immediately, delivery succeeds on the first cycle after Slack returns, and evaluation never stalled

### Requirement: Acknowledgement via Slack command or CLI
The server SHALL expose `POST /telemetry/alerts/{id}/ack`. An `ack <id>` command in the existing Slack slash-command handler and a `task alert:ack ID=<id>` target SHALL both acknowledge through that endpoint, recording when and via which channel the alert was acked. Acknowledging SHALL silence re-notification for that alert while its condition persists; acknowledging an unknown or already-resolved alert id SHALL return an error without side effects.

#### Scenario: Ack from Slack stops the nagging
- **WHEN** the operator sends `ack 42` in Slack for a firing alert
- **THEN** alert 42 records the acknowledgement with channel slack and sends no further re-notifications while still firing

#### Scenario: Ack from the terminal
- **WHEN** the operator runs `task alert:ack ID=42`
- **THEN** the same acknowledgement is recorded with channel cli

#### Scenario: Acking a resolved alert fails cleanly
- **WHEN** an ack arrives for an alert that already resolved
- **THEN** the server returns an error and the alert row is unchanged

### Requirement: No self-watching alert
The retired "server heartbeat stale" alert SHALL NOT be reimplemented in-process; server death remains visible only to the operator (external watcher deferred per ADR-0005).

#### Scenario: Server has no self-heartbeat rule
- **WHEN** the configured v1 rule set is inspected
- **THEN** it contains no rule that alerts on the server's own liveness
