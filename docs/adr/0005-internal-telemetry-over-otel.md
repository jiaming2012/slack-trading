# Internal Telemetry replaces the OpenTelemetry pipeline

The otel-lgtm stack (OTel SDKs → OTLP → collector → Prometheus/Loki/Tempo → Grafana on the Windows box) proved oversized for two low-volume clients — it forced a RAM upgrade — and, decisively, unverifiable: the operator could not tell whether the reporting pipeline itself was working. We replace it with Telemetry, an in-process module of the trading server: metrics are recorded by synchronous function calls into our own registry, persisted to the existing playground Postgres database, and alert rules are evaluated by a server goroutine that posts to Slack directly. All `go.opentelemetry.io/*` and Python `opentelemetry-*` dependencies are removed.

## Considered options

- **Keep the OTel SDKs, repoint OTLP at an internal receiver.** Rejected: it preserves the opaque batch/export write path — the core wound — and requires building a mini-collector that decodes OTLP protobuf for exactly two clients we own. The churn savings were illusory anyway: all Go instruments already sit behind the `src/go/telemetry` package seam, so the full rip swaps one package's internals.

## Consequences

- Distributed tracing (25+ span sites, W3C context propagation Go↔Python) is deleted, not ported. If cross-process debugging is ever needed again, it is a new decision.
- Log storage is dropped: error-rate alerting becomes a logrus hook incrementing an error counter. Full log capture is an explicit non-goal of v1.
- No watcher-of-the-watcher: the server alerts on strategy and datasource staleness, but server death is visible only to a human. The old "Go server heartbeat stale" alert cannot exist in-process and is retired; an external watcher is deferred to a future change, along with the module's possible extraction into its own process (the module boundary is kept clean for that).
- Metrics live in Postgres — the store is SQL-queryable (Metabase, status page, CLI), which is the verifiability property the old stack lacked; the trade is that the trading DB now also carries operational time-series data, bounded by a 30-day prune.
- The otel-lgtm container on the Windows box is decommissioned, reclaiming its RAM; Grafana dashboards and alert rules under `observability/` are archived for reference.
