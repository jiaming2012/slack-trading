# Requirements: v3.0 TradeSignal Framework

## Signal Foundation

- [x] **SIG-01**: TradeSignal struct (Name + Attributes + Timestamp) defined in Go models and proto
- [x] **SIG-02**: Every PlaceOrderRequest includes a signal_id linking back to the originating TradeSignal
- [x] **SIG-03**: Signal attributes use `map[string]interface{}` for flexible, schema-free signal types

## Signal Repository

- [x] **REPO-01**: ISignalRepository interface with in-memory implementation for simulations
- [x] **REPO-02**: ESDBSignalRepository implementation for live environments, writing to per-symbol event streams
- [x] **REPO-03**: Opt-in persistence of sim signals to EventStoreDB via CLI flag

## Datasource Scripts

- [x] **DS-01**: Standalone Python datasource scripts produce TradeSignals via WriteSignal RPC
- [x] **DS-02**: All signals write to a single ordered event stream per symbol; clients filter by name
- [x] **DS-03**: Live datasource scripts run from `__main__`; sim strategies import datasource modules directly

## Strategy Migration

- [x] **MIG-01**: All existing strategies migrated to consume TradeSignals instead of inline signal detection
- [x] **MIG-02**: Original strategy files moved to `deprecated/` folder
- [x] **MIG-03**: Each migration validated with behavioral diff tests against original strategy output

## Replay & Queryability

- [x] **REPLAY-01**: Sim strategies can replay persisted signal streams, gated by playground clock time
- [x] **REPLAY-02**: Integration tests verify replay results match in-memory datasource results
- [x] **QUERY-01**: Signals queryable in EventStoreDB by name, symbol, and timeframe attributes

## RPC & Observability

- [x] **RPC-01**: New gRPC endpoint to view which signals (with timestamps + attributes) were processed by a strategy
- [x] **OBS-01**: Signals visible in OTel/Grafana telemetry framework
- [x] **OBS-02**: Alerting when a strategy's expected TradeSignal is not produced within expected interval

## Future Requirements

(None deferred)

## Out of Scope

- Composite signal framework (complex multi-signal aggregation) -- defer to v3.1 if needed
- Signal backtesting optimizer (grid search over signal parameters) -- future enhancement
- External signal sources (third-party APIs producing signals) -- out of scope for v3.0
- Real-time signal visualization dashboard in Metabase -- existing Grafana telemetry sufficient

## Traceability

| REQ-ID | Phase | Plan | Status |
|--------|-------|------|--------|
| SIG-01 | Phase 17 | 17-01 | Complete |
| SIG-02 | Phase 17 | 17-02 | Complete |
| SIG-03 | Phase 17 | 17-01 | Complete |
| REPO-01 | Phase 18 | 18-01 | Complete |
| REPO-02 | Phase 20 | 20-01, 20-02 | Complete |
| REPO-03 | Phase 18 | 18-02 | Complete |
| DS-01 | Phase 24 | | Pending |
| DS-02 | Phase 24 | | Pending |
| DS-03 | Phase 26 | | Pending |
| MIG-01 | Phase 25 | | Pending |
| MIG-02 | Phase 25 | | Pending |
| MIG-03 | Phase 21 | | Complete |
| REPLAY-01 | Phase 23 | | Complete |
| REPLAY-02 | Phase 23 | | Complete |
| QUERY-01 | Phase 20 | | Complete |
| RPC-01 | Phase 24 | | Pending |
| OBS-01 | Phase 23 | | Complete |
| OBS-02 | Phase 26 | | Pending |
