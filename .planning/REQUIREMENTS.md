# Requirements: v3.0 TradeSignal Framework

## Signal Foundation

- [ ] **SIG-01**: TradeSignal struct (Name + Attributes + Timestamp) defined in Go models and proto
- [ ] **SIG-02**: Every PlaceOrderRequest includes a signal_id linking back to the originating TradeSignal
- [ ] **SIG-03**: Signal attributes use `map[string]interface{}` for flexible, schema-free signal types

## Signal Repository

- [ ] **REPO-01**: ISignalRepository interface with in-memory implementation for simulations
- [ ] **REPO-02**: ESDBSignalRepository implementation for live environments, writing to per-symbol event streams
- [ ] **REPO-03**: Opt-in persistence of sim signals to EventStoreDB via CLI flag

## Datasource Scripts

- [ ] **DS-01**: Standalone Python datasource scripts produce TradeSignals via WriteSignal RPC
- [ ] **DS-02**: All signals write to a single ordered event stream per symbol; clients filter by name
- [ ] **DS-03**: Live datasource scripts run from `__main__`; sim strategies import datasource modules directly

## Strategy Migration

- [ ] **MIG-01**: All existing strategies migrated to consume TradeSignals instead of inline signal detection
- [ ] **MIG-02**: Original strategy files moved to `deprecated/` folder
- [ ] **MIG-03**: Each migration validated with behavioral diff tests against original strategy output

## Replay & Queryability

- [ ] **REPLAY-01**: Sim strategies can replay persisted signal streams, gated by playground clock time
- [ ] **REPLAY-02**: Integration tests verify replay results match in-memory datasource results
- [ ] **QUERY-01**: Signals queryable in EventStoreDB by name, symbol, and timeframe attributes

## RPC & Observability

- [ ] **RPC-01**: New gRPC endpoint to view which signals (with timestamps + attributes) were processed by a strategy
- [ ] **OBS-01**: Signals visible in OTel/Grafana telemetry framework
- [ ] **OBS-02**: Alerting when a strategy's expected TradeSignal is not produced within expected interval

## Future Requirements

(None deferred)

## Out of Scope

- Composite signal framework (complex multi-signal aggregation) — defer to v3.1 if needed
- Signal backtesting optimizer (grid search over signal parameters) — future enhancement
- External signal sources (third-party APIs producing signals) — out of scope for v3.0
- Real-time signal visualization dashboard in Metabase — existing Grafana telemetry sufficient

## Traceability

| REQ-ID | Phase | Plan | Status |
|--------|-------|------|--------|
| SIG-01 | TBD | | Pending |
| SIG-02 | TBD | | Pending |
| SIG-03 | TBD | | Pending |
| REPO-01 | TBD | | Pending |
| REPO-02 | TBD | | Pending |
| REPO-03 | TBD | | Pending |
| DS-01 | TBD | | Pending |
| DS-02 | TBD | | Pending |
| DS-03 | TBD | | Pending |
| MIG-01 | TBD | | Pending |
| MIG-02 | TBD | | Pending |
| MIG-03 | TBD | | Pending |
| REPLAY-01 | TBD | | Pending |
| REPLAY-02 | TBD | | Pending |
| QUERY-01 | TBD | | Pending |
| RPC-01 | TBD | | Pending |
| OBS-01 | TBD | | Pending |
| OBS-02 | TBD | | Pending |
