## Why

ADR-0005: the otel-lgtm stack (OTel SDKs → OTLP → collector → Prometheus/Loki/Tempo → Grafana on the Windows box) proved oversized for two low-volume clients — it forced a RAM upgrade — and, decisively, unverifiable: the operator could not tell whether the reporting pipeline itself was working. The ADR rejects keeping the OTel SDKs and repointing them (it preserves the opaque batch/export write path), and notes that all Go instruments already sit behind the `src/go/telemetry` package seam — so the full rip is one change that swaps that package's internals.

## What Changes

- **Telemetry module (in-process)**: `src/go/telemetry` is rewritten around our own registry — metrics are recorded by synchronous function calls into in-memory counters and labeled gauges. Same instrument set (orders placed/filled/rejected, candles processed, signals generated/consumed, active playgrounds, open orders, uptime), tagged by mode per the CONTEXT.md glossary. No OTel meter, provider, or exporter anywhere in the module.
- **Postgres persistence**: a snapshot writer goroutine persists registry state to the existing playground Postgres database; a prune goroutine enforces the 30-day bound. Every recorded number becomes SQL-queryable — the verifiability property the old stack lacked.
- **Heartbeats**: each strategy and each datasource emits a periodic liveness signal. Python clients stop exporting OTLP gauges and instead report heartbeats to the trading server, which records them in the registry and tracks staleness. The server does not heartbeat to itself.
- **Alerting**: a server goroutine evaluates alert rules over telemetry — stale heartbeats and error-rate spikes (a logrus hook incrementing an error counter replaces log-pipeline alerting) — and posts alerts to Slack directly. The old "Go server heartbeat stale" Grafana alert is retired, not ported: server death is visible only to a human (external watcher deferred, per ADR).
- **Acknowledgement + retry**: an alert keeps re-posting to Slack on a repeat interval until the operator acknowledges it or the condition resolves; a failed Slack post is retried automatically on the next cycle, so delivery retry falls out of the same loop. Acknowledge from Slack (an `ack <id>` reply through the existing slash-command handler) or from the terminal (`task alert:ack ID=<id>`). Unacknowledged alerts are visible in `task telemetry:status`.
- **Tracing deleted, not ported**: all span sites (25+ across Go and Python), W3C trace-context propagation Go↔Python, and the trace/span test suites are removed. If cross-process debugging is ever needed again, it is a new decision.
- **BREAKING — dependency removal**: every `go.opentelemetry.io/*` and `github.com/uptrace/opentelemetry-go-extra/*` module leaves `go.mod`; every `opentelemetry-*` package leaves `src/clients/python/requirements.txt` and `grodt.yml`. `utils.SetupOTelSDK`, the otellogrus/OTel-log bridges, and `engine/otel.py` are deleted.
- **Stack decommission**: the `observability/` Grafana dashboards and alert rules are archived for reference (moved under `deprecated/`); the `observability:*` task targets, `OTEL_SDK_DISABLED` toggles, `test:e2e:otel-candle-metrics`, and the Grafana/OTel sections of `infra:verify` / `infra:refresh-dashboards` are removed. The otel-lgtm container on the Windows box is decommissioned by the operator (follow-up task), reclaiming its RAM.
- **Operator surface**: a `task telemetry:status` target queries the database and shows current heartbeats, latest metrics, and recent alerts (unacknowledged ones flagged); a `task alert:ack` target acknowledges an alert from the terminal (Taskfile entry-point convention).
- Log storage is dropped; full log capture is an explicit non-goal of v1.

## Capabilities

### New Capabilities
- `telemetry-registry`: the in-process metric registry — instruments, synchronous recording, mode tagging, snapshot read-out.
- `telemetry-persistence`: durable telemetry storage in the playground Postgres database — schema, snapshot writer cadence, 30-day prune.
- `telemetry-heartbeats`: strategy and datasource liveness signals — client emission, server ingestion, staleness tracking.
- `telemetry-alerting`: server-side alert rule evaluation (stale heartbeat, error-rate spike), direct Slack delivery, and the acknowledgement lifecycle — re-notify until acked, delivery retry, ack via Slack command or CLI.
- `otel-decommission`: the removal contract — no OTel dependencies in either language, tracing deleted, observability stack archived, task targets retired, operator status command in place.

### Modified Capabilities

_None — no existing spec in `openspec/specs/` covers the OTel pipeline; its behavior was never spec'd, so removal introduces no delta to existing capabilities._

## Impact

- **Go**: `src/go/telemetry/*` (rewritten), `src/go/utils/otel.go` / `otel_logrus_bridge.go` / `telemetry.go` (deleted), `cmd/main.go` (setup/shutdown rewiring), span sites removed across `backtester/{router,rpc,models}`, `marketdata`, `workers`, `api`, `models` (incl. `trace_context.go`), new store code for the telemetry tables, REST ingestion endpoint on the existing :8080 mux.
- **Python**: `engine/otel.py` deleted; `engine/heartbeat.py`, `engine/datasource_heartbeat.py`, `engine/client.py`, `engine/trading_engine.py`, `strategies/base_strategy.py` rewired; OTel test modules replaced by tests of the new reporting path.
- **Database**: new telemetry tables in the playground Postgres DB (auto-migrated); the trading DB now also carries operational time-series data, bounded by the 30-day prune (accepted in ADR-0005).
- **Dependencies**: ~10 Go modules and 6 Python packages removed; none added.
- **Build/tasks**: `taskfile.yml` observability and OTel-related targets removed; `telemetry:status` added.
- **Docs/roadmap**: new card `replace-otel-with-internal-telemetry` under "Live simulation observability" (yellow on draft, green on archive); CLAUDE.md observability/deploy sections updated post-implementation; `verify-grafana-heartbeat-and-stale-alert` roadmap card is obsoleted by this change (Grafana is decommissioned) and will be removed from the map.
- **Operator**: one manual follow-up — stop/remove the otel-lgtm container on the Windows desktop and reclaim its RAM.
