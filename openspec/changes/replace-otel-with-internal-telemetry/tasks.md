## 1. Telemetry registry (Go)

- [x] 1.1 Implement the in-memory registry in `src/go/telemetry`: `Counter` and `Gauge` instrument types with labels, mutex-guarded synchronous recording, `Snapshot()` read-out; unit tests for increment, last-value-per-label-set, and snapshot contents
- [x] 1.2 Re-declare the existing exported instruments (OrdersPlaced, OrdersFilled, OrdersRejected, CandlesProcessed, SignalsGenerated, SignalsConsumed, ActivePlaygrounds, OpenOrders, UptimeSeconds) on registry types; make `telemetry.Init()` self-contained (no OTel provider precondition)
- [x] 1.3 Rewire all Go call sites of the instruments (backtester router/models, marketdata, workers, api) to the registry API, carrying the mode label; `go build ./...` green
- [x] 1.4 Rewire `telemetry.StartHeartbeat` server-stats gauges to the registry (structured log line unchanged)

## 2. Persistence (Postgres)

- [ ] 2.1 Add `telemetry_metrics`, `telemetry_heartbeats`, `telemetry_alerts` GORM models and hook them into startup auto-migration
- [ ] 2.2 Implement the snapshot writer goroutine (default 60s, `TELEMETRY_SNAPSHOT_INTERVAL`) flushing registry series to `telemetry_metrics`; failures log-and-continue; tests with sqlite/testcontainer
- [ ] 2.3 Implement the daily 30-day prune goroutine for `telemetry_metrics`; test that old rows go and young rows stay
- [ ] 2.4 Wire writer + prune startup into `cmd/main.go`

## 3. Heartbeat ingestion

- [ ] 3.1 Implement `POST /telemetry/heartbeat` on the :8080 mux: payload validation (4xx on missing kind/name), registry last-seen update, `telemetry_heartbeats` upsert; handler tests
- [ ] 3.2 Track per-source staleness in the registry (threshold default 90s, `TELEMETRY_HEARTBEAT_STALE_AFTER`); expose to alert rules and status query; tests for fresh→stale transition

## 4. Alerting

- [ ] 4.1 Implement the logrus error-counter hook and register it in `cmd/main.go`; test that Error-level entries increment the counter
- [ ] 4.2 Implement the alert engine goroutine: 30s evaluation, stale-heartbeat rule, error-rate rule (`TELEMETRY_ERROR_WINDOW`/`TELEMETRY_ERROR_THRESHOLD`), firing→(acked)→resolved lifecycle with re-notify-until-acked (`TELEMETRY_ALERT_RENOTIFY_INTERVAL`, default 30m); unit tests for transitions, re-notify, resolution-closes-unacked, and no-self-watch
- [ ] 4.3 Wire Slack delivery through the existing `SlackNotifierClient.SendMessage` with per-cycle delivery retry until a post succeeds (`last_notified_at` semantics); persist every transition to `telemetry_alerts` even when Slack fails; messages carry the alert id + ack instruction
- [ ] 4.4 Implement `POST /telemetry/alerts/{id}/ack` (error on unknown/resolved id), the `ack <id>` case in the Slack slash-command handler, and the `task alert:ack ID=<id>` target; tests for ack-via-slack, ack-via-cli, ack-silences-renotify, ack-after-resolve fails

## 5. Python client rewire

- [ ] 5.1 Rewrite `engine/heartbeat.py` and `engine/datasource_heartbeat.py` to POST to `/telemetry/heartbeat` via `requests` (same public API; warn-and-continue on failure); delete `engine/otel.py`
- [ ] 5.2 Remove `opentelemetry` imports/usage from `engine/client.py`, `engine/trading_engine.py`, `strategies/base_strategy.py`
- [ ] 5.3 Replace OTel test modules (`test_otel.py`, `test_trace_propagation.py`, `test_client_trace_id.py`, `test_trading_engine_otel.py`) with tests of the new reporting path (mocked HTTP); `task test:python` zero failures
- [ ] 5.4 Remove `opentelemetry-*` from `src/clients/python/requirements.txt` and `grodt.yml`

## 6. Tracing and OTel removal (Go)

- [ ] 6.1 Delete all span sites across `backtester/{router,rpc,models}`, `marketdata`, `workers`, `api`, `models`; delete `models/trace_context.go`, `utils/telemetry.go` (trace serialization), and the ESDB `SpanContext` metadata field (replay-tolerant read); update affected tests
- [ ] 6.2 Delete `utils/otel.go`, `utils/otel_logrus_bridge.go`, `utils/otel_test.go`; remove otellogrus/OTel-log hooks and OTel setup/shutdown from `cmd/main.go`
- [ ] 6.3 `go mod tidy`; verify no `go.opentelemetry.io/*` or uptrace module remains in `go.mod`/`go.sum`; repo grep for `opentelemetry` clean outside `deprecated/` and docs
- [ ] 6.4 Gates: `go build ./...`, `task test`, `task lint:package-names`, `task test:no-gorm-leaks` green

## 7. Stack decommission and operator surface

- [ ] 7.1 Move `observability/` to `deprecated/observability/`; delete the `TestLiveCandleMetricPipeline` integration test
- [ ] 7.2 Taskfile: remove `observability:*`, `test:e2e:otel-candle-metrics`, `OTEL_SDK_DISABLED` prefixes, Grafana/OTel sections of `infra:verify`, and `infra:refresh-dashboards`; add telemetry freshness check to `infra:verify`
- [ ] 7.3 Add `task telemetry:status` (heartbeats with staleness, latest snapshot, recent alerts with unacked flagged)
- [ ] 7.4 Docs: update CLAUDE.md deploy/observability sections; add roadmap card `replace-otel-with-internal-telemetry` under "Live simulation observability" (yellow on draft — done at proposal commit; green on archive); remove the obsoleted `verify-grafana-heartbeat-and-stale-alert` card

## 8. Verification

- [ ] 8.1 End-to-end proof: boot server + a Python strategy locally, observe heartbeat rows, metric snapshots, a forced stale alert reaching Slack (or its persisted row), ack it via `task alert:ack` and confirm re-notification stops, and check `task telemetry:status` output
- [ ] 8.2 Full gate sweep: `go build ./...`, `task test`, `task test:python`, `task test:smoke`
- [ ] 8.3 Operator-only: deploy to droplet, run `task infra:verify` + `task telemetry:status`; decommission the otel-lgtm container on the Windows desktop and reclaim its RAM
