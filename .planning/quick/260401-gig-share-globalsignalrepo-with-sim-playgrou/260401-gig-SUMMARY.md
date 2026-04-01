---
phase: quick
plan: 260401-gig
subsystem: backtester-api, python-strategies
tags: [signal-repo, observability, sim-playground]
dependency_graph:
  requires: [globalSignalRepo, write_signal RPC]
  provides: [sim signal visibility, strategy signal writes]
  affects: [GetProcessedSignals, MeanReversionStrategyV2]
tech_stack:
  added: []
  patterns: [shared-repo-injection, try-except-rpc-guard]
key_files:
  created: []
  modified:
    - src/go/backtester-api/router/grpc.go
    - src/clients/python/strategies/mean_reversion_v2.py
decisions:
  - Sim playgrounds share globalSignalRepo (single playground per sim, no cursor issues)
  - write_signal failures caught and logged, never interrupt trading logic
metrics:
  duration: 1m31s
  completed: 2026-04-01
  tasks: 2/2
---

# Quick Task 260401-gig: Share globalSignalRepo with Sim Playgrounds Summary

Sim playgrounds now use the server's shared globalSignalRepo so WriteSignal writes from Python strategies are queryable via GetProcessedSignals, and MeanReversionStrategyV2 calls write_signal() on every detected signal for observability.

## Completed Tasks

| # | Task | Commit | Key Changes |
|---|------|--------|-------------|
| 1 | Share globalSignalRepo with sim playgrounds in Go server | 134a3c4 | Changed default case from NewInMemorySignalRepository() to s.globalSignalRepo |
| 2 | Wire MeanReversionStrategyV2 to call write_signal() on signal detection | 4754944 | Added write_signal() loop in _process_htf_candle() with try/except guard |

## Changes Made

### Go Server (grpc.go)
- Line 1513: Default case in CreatePlayground signal repo assignment changed from `models.NewInMemorySignalRepository()` to `s.globalSignalRepo`
- This means sim playgrounds now share the same signal repository that WriteSignal and GetProcessedSignals use

### Python Strategy (mean_reversion_v2.py)
- Added a new loop before the existing signal processing loop in `_process_htf_candle()`
- For each detected signal, calls `self.playground.write_signal()` with signal_key, symbol, timestamp, and source attributes
- Wrapped in try/except so RPC failures are logged as warnings without breaking trading logic

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

1. Go build passes: `go build ./src/go/backtester-api/...` -- PASS
2. Python import succeeds: `import strategies.mean_reversion_v2` -- PASS
3. Grep confirms globalSignalRepo wired in default case -- CONFIRMED (line 1513)
4. Grep confirms write_signal called from strategy -- CONFIRMED (lines 257, 264)

## Known Stubs

None.

## Self-Check: PASSED
