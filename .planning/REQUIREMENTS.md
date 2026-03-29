# Requirements: v1.1 Dashboard Enhancements

## Metric Labels & Filtering

- [ ] **LABEL-01**: All Go-side OTel metrics emit client_id as a label alongside playground_id
- [ ] **LABEL-02**: Grafana playground dropdown shows client_id (fallback to playground_id if no client_id set)
- [ ] **LABEL-03**: Signals Generated panel filterable by signal_type name
- [ ] **LABEL-04**: Candles Processed panel filterable by candle symbol

## Panel Improvements

- [ ] **PANEL-01**: Active Playgrounds lists client_id/playground_id instead of a count
- [ ] **PANEL-02**: Python Strategy Heartbeat shows per-playground status, not binary alive/dead
- [ ] **PANEL-03**: Open Orders panel shows order symbol + quantity detail
- [ ] **PANEL-04**: order_filled log events include actual quantity (fix missing qty bug)
- [ ] **PANEL-05**: Recent Order Events panel includes trace_id
- [ ] **PANEL-06**: Position Details panel includes trace_id

## Per-Strategy Dashboards

- [ ] **STRAT-01**: Separate Grafana dashboard per strategy type (mean_reversion, covered_call, etc.) with strategy-specific panels

## Future Requirements

(None deferred)

## Out of Scope

- Custom Grafana plugins — use built-in panels only
- Alerting changes — covered in v1.0, no changes this milestone
- Mobile/responsive dashboard layout — desktop-first

## Traceability

| REQ-ID | Phase | Plan | Status |
|--------|-------|------|--------|
| LABEL-01 | | | Pending |
| LABEL-02 | | | Pending |
| LABEL-03 | | | Pending |
| LABEL-04 | | | Pending |
| PANEL-01 | | | Pending |
| PANEL-02 | | | Pending |
| PANEL-03 | | | Pending |
| PANEL-04 | | | Pending |
| PANEL-05 | | | Pending |
| PANEL-06 | | | Pending |
| STRAT-01 | | | Pending |
