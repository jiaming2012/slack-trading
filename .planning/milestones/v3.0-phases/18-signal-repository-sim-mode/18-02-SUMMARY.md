---
phase: 18-signal-repository-sim-mode
plan: 02
subsystem: backtester-api, proto, eventproducers
tags: [signal-delivery, proto, tick-loop, esdb-persistence]
dependency_graph:
  requires: [18-01]
  provides: [signal-tick-delivery, esdb-signal-persistence]
  affects: [playground, grpc-router, tick-delta, esdb-producer]
tech_stack:
  added: []
  patterns: [clock-gated-delivery, batch-on-save, pub-sub-esdb-persistence]
key_files:
  created: []
  modified:
    - src/go/playground.proto
    - src/go/playground/playground.pb.go
    - src/go/playground/playground.twirp.go
    - src/clients/python/rpc/playground_pb2.py
    - src/clients/python/rpc/playground_twirp.py
    - src/go/backtester-api/models/tick_delta.go
    - src/go/backtester-api/models/playground.go
    - src/go/backtester-api/router/grpc.go
    - src/go/eventproducers/esdb_producer.go
decisions:
  - Attributes map[string]interface{} converted to map[string]string via fmt.Sprintf for proto compat
  - ESDB persistence uses existing pub/sub pattern (PublishAndSaveEvent) rather than direct ESDB access
  - EsdbProducer subscription added for TradeSignalEventName to handle signal writes
metrics:
  duration: 5m50s
  completed: "2026-03-30T22:06:19Z"
  tasks_completed: 3
  tasks_total: 3
  files_modified: 9
---

# Phase 18 Plan 02: Signal Tick Delivery & ESDB Persistence Summary

Wire signal delivery through simulation tick loop with ESDB batch persistence on save.

**One-liner:** Clock-gated signal delivery via simulateTick with proto conversion in NextTick and batch ESDB persistence in SavePlayground.

## What Was Done

### Task 1: Add new_signals to proto TickDelta and regenerate stubs
- Added `repeated TradeSignalProto new_signals = 11` to TickDelta message in playground.proto
- Regenerated Go stubs (playground.pb.go, playground.twirp.go) and Python stubs (playground_pb2.py, playground_twirp.py)
- **Commit:** 41dd358

### Task 2: Wire signal delivery in Playground simulateTick and TickDelta
- Added `NewSignals []*eventmodels.TradeSignal` field to TickDelta Go struct
- Added `signalRepo ISignalRepository` field to Playground struct with `json:"-" gorm:"-"` tags
- Added `SetSignalRepo` / `GetSignalRepo` methods following existing setter pattern
- Drains `signalRepo.ReadPending(p.clock.CurrentTime)` after clock.Add in simulateTick (no lookahead)
- Includes NewSignals in returned TickDelta; nil-safe for existing playgrounds without signalRepo
- **Commit:** aec188d

### Task 3: Convert signals to proto in NextTick and batch-persist in SavePlayground
- Added signal-to-proto conversion in NextTick handler (ID, Name, Symbol, Timestamp via timestamppb, Attributes via fmt.Sprintf)
- Included NewSignals in proto TickDelta response
- Added ESDB batch persistence in SavePlayground: drains signalRepo.GetAll(), publishes each via eventpubsub.PublishAndSaveEvent
- Added EsdbProducer subscription to TradeSignalEventName for handling signal ESDB writes
- **Commit:** 78dfad7

## Deviations from Plan

None -- plan executed exactly as written.

## Verification Results

- Full project builds without errors (`go build ./src/go/...`)
- All Signal Repository unit tests pass (6/6 from Plan 01)
- Router tests pass
- Pre-existing test failures in models (env-dependent: loads .env from unrelated project path) -- not caused by these changes

## Decisions Made

1. **Attributes conversion**: Used `fmt.Sprintf("%v", v)` for `map[string]interface{}` to `map[string]string` proto conversion, matching D-02 design decision
2. **ESDB persistence pattern**: Used existing `PublishAndSaveEvent` pub/sub pattern (same as all other ESDB writers) rather than direct ESDB client access, maintaining architectural consistency
3. **EsdbProducer subscription**: Added `TradeSignalEventName` subscription alongside existing event subscriptions in esdb_producer.go Start()

## Known Stubs

None -- all data flows are fully wired.

## Self-Check: PASSED
