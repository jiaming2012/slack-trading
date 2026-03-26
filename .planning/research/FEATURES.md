# Feature Landscape: Trading Platform Observability

**Domain:** Full-stack observability for event-driven trading system
**Researched:** 2026-03-25

## Table Stakes

Features the operator expects. Missing = observability feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Structured log aggregation | Can't ssh into k8s pods to tail logs. Logs must be searchable. | Low | Loki with OTLP ingestion. Go already uses logrus (JSON), Python has structlog. |
| Request tracing (Go server) | Need to trace RPC calls (CreatePlayground, NextTick, PlaceOrder) through the server | Low | OTel spans already exist in ~15 files. Just initialize the provider. |
| Infrastructure metrics | CPU, memory, goroutine count, GC pressure | Low | `runtime.Start()` from otel contrib already in go.mod. One line of code. |
| HTTP request metrics | Latency, error rate, throughput per endpoint | Low | `otelhttp` middleware wraps Gorilla mux and Twirp handler. Already in go.mod. |
| Centralized dashboard | Single pane of glass for all telemetry | Medium | Grafana with Loki + Tempo + Prometheus data sources. Provisioning via YAML. |
| Log-trace correlation | Click from a log line to the trace that produced it | Low | OTel automatically injects trace_id into logs via otellogrus hook. Grafana natively links Loki to Tempo. |
| Service health check | "Is the server up?" at a glance | Low | HTTP endpoint metrics show request rate. Zero requests = likely down. |

## Differentiators

Features that solve the core problem: "Is the strategy actually running?"

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Strategy heartbeat | Periodic metric gauge + log line proving the Python strategy is alive and polling | Medium | Custom OTel gauge in Python client, emitted every N ticks. Grafana alert when stale. This is the killer feature. |
| Order lifecycle tracing | Trace an order from Python strategy decision through RPC call to Go server validation to fill/reject | Medium | Spans across both processes. Requires trace context propagation via HTTP headers (W3C traceparent). |
| Market data flow visibility | See candle arrivals, tick processing rate, data gaps | Medium | Custom metrics (candle count, tick latency) + structured logs on data gaps. |
| Strategy decision logging | Why did the strategy place (or not place) an order? | Medium | Structured logs from Python with indicator values, signal evaluation, decision outcome. |
| Quiet period indicator | Dashboard clearly shows "no trades because no signals" vs "strategy crashed" | Medium | Combines heartbeat (alive) + tick metrics (processing) + trade count (zero is OK if heartbeat is active). |
| Error spike alerting | Grafana alert when error log rate exceeds threshold | Low | LogQL alert rule on error rate. |
| Playground session tracing | Group all telemetry for a single playground/simulation session | Medium | Use playground UUID as a resource attribute on all spans/metrics/logs for that session. |

## Anti-Features

Features to explicitly NOT build.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Distributed tracing into external APIs (Polygon, Tradier) | We don't control those services. Adding trace headers to external calls is meaningless and potentially breaks API contracts. | Instrument our side of the call: measure latency, log response codes, trace our handler. |
| Custom Grafana plugins | High effort, fragile across Grafana upgrades, unnecessary when built-in panels cover all needs. | Use built-in panels: time series, stat, table, logs, trace viewer. |
| PagerDuty/OpsGenie integration | Over-engineering for a single-operator trading platform. | Grafana native alerts (email, Slack webhook -- already have Slack integration). |
| Real-time streaming dashboards | Grafana auto-refresh at 5-10s intervals is sufficient. WebSocket-based real-time adds complexity. | Set Grafana dashboard auto-refresh to 10s. |
| Log-based metrics (deriving metrics from log content) | Fragile, expensive at scale, hard to maintain. | Emit proper OTel metrics from code. Use logs for context, metrics for numbers. |
| Full Python auto-instrumentation | `opentelemetry-instrument` wraps everything but adds noise. Strategy code needs targeted, meaningful spans. | Manual instrumentation of key decision points in trading engine. |

## Feature Dependencies

```
TracerProvider init (Go) -> All Go tracing features
MeterProvider init (Go) -> All Go metrics features
OTel Collector running -> All telemetry reaches backends
Loki running -> Log search, log-based alerts
Tempo running -> Trace viewing, log-trace correlation
Prometheus running -> Metric dashboards, metric-based alerts
Grafana running -> All dashboards and alerts

Go server instrumented -> Python client instrumentation (validate pipeline first)
Heartbeat metric (Python) -> Heartbeat stale alert (Grafana)
Order lifecycle spans -> Order lifecycle dashboard
Strategy decision logs -> Strategy decision dashboard
```

## MVP Recommendation

Prioritize (in order):
1. **TracerProvider + MeterProvider initialization** -- unlocks all existing spans and runtime metrics with near-zero code
2. **Local observability backend** -- `grafana/otel-lgtm` Docker image, one command
3. **HTTP request metrics** -- otelhttp middleware on Gorilla mux + Twirp, proves pipeline end-to-end
4. **Strategy heartbeat** -- the core differentiator, answers "is it running?"

Defer:
- **Order lifecycle tracing**: Requires cross-process trace propagation. Do after single-process tracing works.
- **Production deployment (Digital Ocean)**: Get it right locally first.
- **Grafana alerts**: Build dashboards first, add alerts once you understand the data patterns.
- **Python auto-instrumentation**: Manual instrumentation of key points is more valuable than blanket coverage.

---
*Feature landscape for: Trading platform observability*
*Researched: 2026-03-25*
