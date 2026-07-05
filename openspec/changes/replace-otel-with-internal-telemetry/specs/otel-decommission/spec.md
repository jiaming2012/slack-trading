## ADDED Requirements

### Requirement: All OpenTelemetry dependencies removed
After this change, `go.mod` SHALL contain no direct `go.opentelemetry.io/*` or `github.com/uptrace/opentelemetry-go-extra/*` modules, no Go source file in the repository SHALL import an OpenTelemetry package, and `src/clients/python/requirements.txt` and `grodt.yml` SHALL contain no `opentelemetry-*` packages. Indirect modules pulled in solely by third-party test infrastructure (testcontainers → docker client) are out of scope — they are not our instrumentation and carry no export pipeline. A source grep for `opentelemetry` / `go.opentelemetry.io` SHALL return no hits outside `deprecated/` and the OpenSpec/docs record.

#### Scenario: Go module graph is clean
- **WHEN** `go mod tidy` runs and `go.mod` is inspected
- **THEN** every remaining OpenTelemetry module is marked indirect and `go mod why` attributes it to testcontainers/docker, not to our packages

#### Scenario: Python dependency manifests are clean
- **WHEN** `requirements.txt` and `grodt.yml` are inspected
- **THEN** no `opentelemetry-*` entry remains

### Requirement: Distributed tracing deleted, not ported
All span creation sites, the OTel SDK setup (`utils.SetupOTelSDK`), the otellogrus and OTel-log logrus hooks, W3C trace-context serialization (`models.TraceContext`, `utils.SerializeTraceContext`/`DeserializeTraceContext`), the ESDB event-metadata span-context field, and Python trace propagation SHALL be deleted. Replay of previously-persisted ESDB events SHALL remain unaffected (the obsolete metadata field is ignored on read). `ClientRequestID` on orders SHALL be untouched — it is order idempotency, not tracing.

#### Scenario: Server runs without tracing
- **WHEN** the server starts and serves RPC and REST traffic after the change
- **THEN** no tracer, span, or trace-context code executes and behavior is otherwise unchanged

#### Scenario: Old ESDB events still replay
- **WHEN** the ESDB consumer replays events persisted before this change (whose metadata carries a span-context field)
- **THEN** events are processed normally with the field ignored

### Requirement: Observability stack archived and task targets retired
The `observability/` directory (Grafana dashboards, alerting rules, docker-compose) SHALL move to `deprecated/observability/` for reference. `taskfile.yml` SHALL drop the `observability:*` targets, the `test:e2e:otel-candle-metrics` target and its integration test, all `OTEL_SDK_DISABLED` prefixes, and the Grafana/OTel-collector sections of `infra:verify` and `infra:refresh-dashboards`. Decommissioning the otel-lgtm container on the Windows desktop is an operator-only follow-up task recorded in this change.

#### Scenario: Archived stack is out of the active tree
- **WHEN** the repo is inspected after the change
- **THEN** `observability/` exists only under `deprecated/` and no task target references the otel-lgtm stack

#### Scenario: Task list has no observability targets
- **WHEN** `task --list` runs
- **THEN** no `observability:*` or OTel-related target appears

### Requirement: Operator status command
A `task telemetry:status` target SHALL query the playground database and print current heartbeat sources with seconds-since-last-beat and stale flags, the latest metric snapshot, and the most recent alerts with unacknowledged ones flagged — answering "is the strategy actually running?" from the terminal. `infra:verify` SHALL check that `telemetry_metrics` has rows fresher than two snapshot intervals in place of the removed Grafana checks.

#### Scenario: Operator checks liveness during a quiet period
- **WHEN** the operator runs `task telemetry:status` while a live simulation is running but not trading
- **THEN** the output shows the strategy's heartbeat as fresh, its latest metrics, and any recent alerts

#### Scenario: Post-deploy verification
- **WHEN** `task infra:verify` runs after a deploy
- **THEN** it fails if telemetry snapshots are not being written
