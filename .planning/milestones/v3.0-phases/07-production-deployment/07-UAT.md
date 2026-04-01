---
status: complete
phase: 07-production-deployment
source: [07-01-SUMMARY.md, 07-02-SUMMARY.md]
started: 2026-03-28T04:30:00Z
updated: 2026-03-28T04:30:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Grafana login and dashboard
expected: Open http://159.89.226.131:3000, see login page, log in with admin/grodt2026, home dashboard is "Grodt Live Simulation"
result: pass

### 2. Go server heartbeat visible
expected: "Go Server Uptime" stat panel shows a value in seconds (not "No data")
result: pass

### 3. Active playgrounds count
expected: "Active Live Playgrounds" stat panel shows at least 1
result: pass

### 4. Orders placed metric
expected: "Orders Placed / Filled" panel shows your order count (e.g. 3+ placed)
result: pass

### 5. Python strategy heartbeat
expected: "Python Strategy Heartbeat" panel shows "Alive" (value 1) while strategy is running
result: pass

### 6. Recent order events in Loki
expected: "Recent Order Events" table shows rows with event/symbol/side/qty from your PlaceOrder calls
result: pass

### 7. Twirp RPC accessible
expected: POST to http://159.89.226.131:5051/twirp/playground.PlaygroundService/GetPlaygrounds returns playground list
result: pass

### 8. Firewall blocks unauthorized ports
expected: curl to http://159.89.226.131:8080 times out (REST port blocked by firewall)
result: pass

## Summary

total: 8
passed: 8
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps
