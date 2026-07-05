## Context

ADR-0005 decided the full rip: the otel-lgtm stack is oversized and unverifiable for two low-volume clients. All Go instruments already sit behind the `src/go/telemetry` seam, so the swap is one change. Current state: Go server initializes the OTel SDK (`utils.SetupOTelSDK`), records metrics via OTel instruments in `src/go/telemetry`, starts spans at ~36 sites, bridges logrus to OTLP; Python clients (`engine/otel.py`) export OTLP gauges for strategy/datasource heartbeats and propagate W3C trace context; ESDB event metadata carries a serialized span context. The CONTEXT.md glossary fixes the vocabulary: **Telemetry** (in-server module, every number queryable in the platform DB, emitted in every mode tagged by mode), **Heartbeat** (strategy/datasource liveness; stale → Alert; the server does not heartbeat to itself), **Alert** (server-pushed Slack notification; no external alerting system).

## Goals / Non-Goals

**Goals:**
- Every recorded metric, heartbeat, and alert is a row the operator can `SELECT` — pipeline verifiability by construction.
- Synchronous in-process recording; no batch/export write path anywhere.
- Zero OTel dependencies in `go.mod`, `requirements.txt`, `grodt.yml` when done.
- Alerting (stale heartbeat, error-rate spike) delivered to Slack by the server itself.
- Module boundary of `src/go/telemetry` stays clean for possible future extraction into its own process.

**Non-Goals:**
- Log storage / full log capture (explicit v1 non-goal in ADR-0005).
- Distributed tracing replacement — deleted, not ported; a future need is a new decision.
- Watcher-of-the-watcher: server death stays visible only to a human; external watcher deferred.
- Dashboards, Metabase, status page — the SQL queryability enables them later; only `task telemetry:status` ships now.

## Decisions

**D1 — Registry shape.** `src/go/telemetry` owns an in-memory registry: `Counter` (monotonic, labeled) and `Gauge` (last-value, labeled) instrument types, recorded synchronously under a mutex keyed by (name, sorted label set). The existing exported instrument variables keep their names (`OrdersPlaced`, `ActivePlaygrounds`, …) so call-site churn is mechanical (attribute syntax only). Mode is a label on every series, per the glossary. `Registry.Snapshot()` returns point-in-time rows for the writer. *Alternative rejected:* keeping OTel instrument types over an internal reader — preserves the opaque write path ADR-0005 rejects.

**D2 — Persistence.** A snapshot writer goroutine flushes the registry to a `telemetry_metrics` table (`recorded_at`, `name`, `labels` jsonb, `kind`, `value`) every 60s (env-overridable). Counters persist cumulative values; rates are a SQL window over snapshots. Heartbeats get their own `telemetry_heartbeats` table (`source_kind` strategy|datasource, `name`, `meta` jsonb, `last_seen_at`, `beat_count`) written as upserts on ingest — a heartbeat's arrival must not wait for a snapshot tick. A prune goroutine deletes `telemetry_metrics` rows older than 30 days daily (heartbeat rows are one-per-source upserts and don't grow). Tables auto-migrate at startup alongside the existing GORM migrations. *Alternative rejected:* per-record synchronous DB writes — write amplification inside the tick loop for no verifiability gain.

**D3 — Client → server transport.** Python strategies/datasources report heartbeats via `POST /telemetry/heartbeat` (JSON) on the existing :8080 REST mux. The Python `StrategyHeartbeat` / `DatasourceHeartbeat` classes keep their public API (`start/record_tick/record_check/set_state/stop`) but their daemon threads post JSON with `requests` (already a dependency) instead of recording OTel gauges. A failed post logs a warning and the thread continues — client trading behavior never depends on telemetry delivery. *Alternative rejected:* extending `playground.proto` — telemetry is operational, not trading API; a proto change forces stub regeneration in both languages for a two-field payload.

**D4 — Alert engine.** A server goroutine evaluates rules every 30s over the in-memory registry (not the DB — evaluation must survive DB slowness):
- *Stale heartbeat*: any registered strategy/datasource whose `last_seen_at` is older than 90s (3 missed beats; env-overridable) fires.
- *Error-rate spike*: a logrus hook increments an `errors_total` counter (replacing log-pipeline alerting); the rule fires when the count in a 5-minute window exceeds a threshold (default 10, env-overridable).
Alerts have a firing→(acknowledged)→resolved lifecycle persisted in a `telemetry_alerts` table (`rule`, `subject`, `message`, `fired_at`, `acked_at`, `acked_via`, `resolved_at`, `last_notified_at`) — alerts are telemetry too. The retired "server heartbeat stale" Grafana alert has no in-process equivalent by design.

**D4a — Acknowledgement and re-notify-until-acked.** A firing alert re-posts to Slack every re-notify interval (default 30 minutes, `TELEMETRY_ALERT_RENOTIFY_INTERVAL`) until the operator acknowledges it or the condition resolves. Acknowledging silences re-notification for that alert while the condition persists; resolution closes it (with a resolution notification) whether or not it was acked. Delivery retry falls out of the same loop: each evaluation cycle (30s) checks "should this alert have an outstanding notification?" — a failed Slack post simply leaves `last_notified_at` unset, so the next cycle retries until one succeeds, then the normal re-notify interval applies. Delivery reuses `SlackNotifierClient.SendMessage`; every Slack message includes the alert id and the ack instruction.

**D4b — Ack channels.** Acking is one REST call — `POST /telemetry/alerts/{id}/ack` — with two front doors: an `ack <id>` case added to the existing Slack slash-command switch (`src/go/api/slack`), and a `task alert:ack ID=<id>` target that curls the endpoint. Both record `acked_via` (slack|cli). Unacked alerts are flagged in `task telemetry:status`. *Alternative rejected:* Slack interactive buttons — requires Slack app interactivity callbacks and signing-secret verification we don't have; the slash-command handler already exists.

**D5 — Tracing deletion boundaries.** Deleted: all `tracer.Start`/span sites, `utils.SetupOTelSDK`, `utils/otel_logrus_bridge.go`, the otellogrus hook, `models.TraceContext`, `utils.SerializeTraceContext`/`DeserializeTraceContext`, the `SpanContext` field on `EsdbEvent`/`EsdbMetadata` (old persisted ESDB events keep their metadata bytes; JSON unmarshal simply ignores the field — replay is unaffected), Python `engine/otel.py` and trace-propagation code, and the OTel-specific test modules in both languages. **Explicitly untouched:** `ClientRequestID` on orders — it is idempotency plumbing, not tracing.

**D6 — Configuration.** Hardcoded defaults with env overrides only for operator-tunable knobs: `TELEMETRY_SNAPSHOT_INTERVAL`, `TELEMETRY_HEARTBEAT_STALE_AFTER`, `TELEMETRY_ERROR_WINDOW`, `TELEMETRY_ERROR_THRESHOLD`. No new config file section; the options-config YAML stays trading-only.

**D7 — Operator surface.** `task telemetry:status` runs SQL against the playground DB and prints: each heartbeat source with seconds-since-last-beat and stale flag, the latest metric snapshot, and the last 10 alerts. This is the "is the strategy actually running?" command and the post-deploy verification step replacing the Grafana sections of `infra:verify`.

**D8 — Decommission mechanics.** `observability/` moves to `deprecated/observability/` (archived for reference, per ADR). Taskfile: `observability:*` targets, `test:e2e:otel-candle-metrics` (and the integration test it runs), `OTEL_SDK_DISABLED` prefixes, and the Grafana/OTel-collector sections of `infra:verify`/`infra:refresh-dashboards` are removed; `infra:verify` gains a freshness check that `telemetry_metrics` has rows newer than two snapshot intervals. Stopping the otel-lgtm container on the Windows box is an operator-only task.

## Risks / Trade-offs

- [Operational time-series in the trading DB] → bounded by the 30-day prune and one batched write per snapshot interval; row volume ≈ instrument-series count per minute, trivial at this scale.
- [No tracing when a cross-process bug appears] → accepted in ADR-0005; the decision to rebuild tracing would be a new ADR.
- [Slack outage delays alerts] → alert rows persist immediately; the evaluation loop retries delivery every cycle until a post succeeds; `task telemetry:status` shows undelivered/unacked alerts meanwhile.
- [Re-notify nagging during a known incident] → one `ack` silences that alert while the condition persists; the interval is env-tunable.
- [Server death invisible] → accepted; external watcher deferred to a future change; module boundary kept clean for it.
- [Heartbeat upsert on every beat (30s × few sources)] → negligible write load; if sources multiply, batch in the ingest handler later without changing the wire contract.
- [Big-bang removal risks missed references] → gate on `go build ./...`, full grep for `opentelemetry`/`go.opentelemetry.io` returning zero hits outside `deprecated/`, and the Go + Python test suites green.

## Migration Plan

1. Land the module + endpoint + alerting with OTel still present (single change, ordered tasks) — tables auto-migrate on first boot.
2. Rip tracing/OTel and rewire Python in the same change; `go mod tidy` drops the modules.
3. Deploy to the droplet; verify with `task infra:verify` + `task telemetry:status`.
4. Operator decommissions otel-lgtm on the Windows box (manual follow-up task; reclaims RAM).
Rollback: `git revert` of the change commits; the telemetry tables are additive and harmless to leave in place.

## Open Questions

_None blocking — thresholds and intervals ship as defaults with env overrides; tuning is operational._
