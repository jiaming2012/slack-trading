# Requirements: v1.1 Dashboard Enhancements

## Metric Labels & Filtering

- [x] **LABEL-01**: All Go-side OTel metrics emit client_id as a label alongside playground_id
- [x] **LABEL-02**: Grafana playground dropdown shows client_id (fallback to playground_id if no client_id set)
- [ ] **LABEL-03**: Signals Generated panel filterable by signal_type name
- [ ] **LABEL-04**: Candles Processed panel filterable by candle symbol

## Panel Improvements

- [x] **PANEL-01**: Active Playgrounds lists client_id/playground_id instead of a count
- [ ] **PANEL-02**: Python Strategy Heartbeat shows per-playground status, not binary alive/dead
- [x] **PANEL-03**: Open Orders panel shows order symbol + quantity detail
- [x] **PANEL-04**: order_filled log events include actual quantity (fix missing qty bug)
- [x] **PANEL-05**: Recent Order Events panel includes trace_id
- [x] **PANEL-06**: Position Details panel includes trace_id

## Per-Strategy Dashboards

- [ ] **STRAT-01**: Separate Grafana dashboard per strategy type (mean_reversion, covered_call, etc.) with strategy-specific panels

## Future Requirements

(None deferred)

## Out of Scope

- Custom Grafana plugins -- use built-in panels only
- Alerting changes -- covered in v1.0, no changes this milestone
- Mobile/responsive dashboard layout -- desktop-first

## Traceability

| REQ-ID | Phase | Plan | Status |
|--------|-------|------|--------|
| LABEL-01 | Phase 8 | 08-01 | Complete |
| LABEL-02 | Phase 9 | 09-01 | Complete |
| LABEL-03 | Phase 10 | | Pending |
| LABEL-04 | Phase 10 | | Pending |
| PANEL-01 | Phase 9 | 09-01 | Complete |
| PANEL-02 | Phase 10 | | Pending |
| PANEL-03 | Phase 9 | 09-01 | Complete |
| PANEL-04 | Phase 8 | 08-01 | Complete |
| PANEL-05 | Phase 9 | 09-01 | Complete |
| PANEL-06 | Phase 9 | 09-01 | Complete |
| STRAT-01 | Phase 11 | | Pending |
