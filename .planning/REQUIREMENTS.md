# Requirements: v2.0 Metabase Analytics

## Infrastructure

- [x] **INFRA-01**: Metabase v0.59.4 deployed via docker-compose on existing DO droplet (port 3001)
- [x] **INFRA-02**: Metabase uses dedicated Postgres database (`metabaseappdb`) — not H2
- [x] **INFRA-03**: JVM memory capped at 768MB with Docker mem_limit 1.5GB
- [x] **INFRA-04**: Read-only Postgres user (`metabase_ro`) for trading DB queries
- [x] **INFRA-05**: Cloud Firewall updated to allow TCP 3001

## Analytics Schema

- [x] **SCHEMA-01**: Composite indexes on order_records/trade_records for analytics queries
- [x] **SCHEMA-02**: SQL views for P&L, win rate, profit factor (matching playground_metrics.py logic)
- [ ] **SCHEMA-03**: `spread_groups` and `spread_group_legs` tables for multi-leg option grouping
- [ ] **SCHEMA-04**: SQL view for spread-aware P&L (net spread profit, not per-leg)

## Backtest Persistence

- [x] **PERSIST-01**: Simulator playgrounds persist order_records and trade_records to Postgres on completion
- [x] **PERSIST-02**: `backtest_runs` summary table (final_balance, win_rate, profit_factor, parameters JSONB)

## Dashboards

- [x] **DASH-01**: Trading performance dashboard: P&L over time, win rate, profit factor, gross profit/loss
- [x] **DASH-02**: Slippage analysis dashboard: open/close/total slippage per playground
- [ ] **DASH-03**: Strategy comparison dashboard: compare backtests by parameters and strategy types
- [x] **DASH-04**: Portfolio analytics: position history, per-symbol/asset-class breakdown
- [ ] **DASH-05**: Spread analytics dashboard: spread P&L, spread win/loss ratio

## Spread Analytics

- [ ] **SPREAD-01**: Go server registers spread legs via spread_group_key attribute on PlaceMultiLegOrder
- [ ] **SPREAD-02**: Python strategies emit spread_group_key for multi-leg orders

## Future Requirements

(None deferred)

## Out of Scope

- Metabase Pro/Enterprise features — OSS only
- Real-time streaming dashboards — Metabase is for historical analytics (Grafana handles real-time)
- Mobile Metabase app — desktop browser only
- Replacing playground_metrics.py — it continues to work for CLI usage; Metabase provides the same analytics via SQL

## Traceability

| REQ-ID | Phase | Plan | Status |
|--------|-------|------|--------|
| INFRA-01 | Phase 12 | | Pending |
| INFRA-02 | Phase 12 | | Pending |
| INFRA-03 | Phase 12 | | Pending |
| INFRA-04 | Phase 12 | | Pending |
| INFRA-05 | Phase 12 | | Pending |
| SCHEMA-01 | Phase 13 | | Pending |
| SCHEMA-02 | Phase 13 | | Pending |
| SCHEMA-03 | Phase 16 | | Pending |
| SCHEMA-04 | Phase 16 | | Pending |
| PERSIST-01 | Phase 15 | | Pending |
| PERSIST-02 | Phase 15 | | Pending |
| DASH-01 | Phase 14 | | Pending |
| DASH-02 | Phase 14 | | Pending |
| DASH-03 | Phase 15 | | Pending |
| DASH-04 | Phase 14 | | Pending |
| DASH-05 | Phase 16 | | Pending |
| SPREAD-01 | Phase 16 | | Pending |
| SPREAD-02 | Phase 16 | | Pending |
