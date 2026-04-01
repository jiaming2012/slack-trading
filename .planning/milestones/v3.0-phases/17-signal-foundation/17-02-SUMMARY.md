---
phase: 17-signal-foundation
plan: 02
subsystem: api
tags: [uuid, gorm, twirp, proto, signal-id, order-lifecycle]

# Dependency graph
requires:
  - phase: 17-signal-foundation (plan 01)
    provides: TradeSignal struct, SignalName registry, proto TradeSignalProto message, signal_id on PlaceOrderRequest and Order
provides:
  - SignalID nullable UUID field on OrderRecord with GORM index
  - SignalID field on CreateOrderRequest for order lifecycle passthrough
  - PlaceOrder RPC handler parses optional signal_id into *uuid.UUID
  - convertOrder maps OrderRecord.SignalID to proto Order.SignalId
  - commitOrderRecord sets SignalID post-construction
  - RecordSignal deprecation comment
affects: [18-signal-repository, 19-rpc-endpoints, 21-strategy-migration]

# Tech tracking
tech-stack:
  added: []
  patterns: [post-construction field assignment for optional fields, nullable UUID foreign key with index]

key-files:
  created: []
  modified:
    - src/go/backtester-api/models/order_record.go
    - src/go/backtester-api/models/create_order_request.go
    - src/go/backtester-api/router/grpc.go
    - src/go/data/database_service.go

key-decisions:
  - "SignalID set post-construction in commitOrderRecord to avoid modifying PopulateOrderRecord's 20+ parameter signature"
  - "RecordSignal marked deprecated with comment only (body unchanged) per D-04 design decision"

patterns-established:
  - "Post-construction assignment: optional fields added to OrderRecord are set after PopulateOrderRecord call in commitOrderRecord"
  - "Nullable UUID foreign key: *uuid.UUID with gorm index for optional cross-entity references"

requirements-completed: [SIG-02]

# Metrics
duration: 3min
completed: 2026-03-30
---

# Phase 17 Plan 02: Signal ID Order Wiring Summary

**Nullable signal_id UUID wired through full order lifecycle: PlaceOrder RPC -> CreateOrderRequest -> commitOrderRecord -> OrderRecord -> convertOrder proto response**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-30T20:29:55Z
- **Completed:** 2026-03-30T20:32:48Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- OrderRecord has nullable SignalID field with GORM index (auto-migrates on next server startup)
- PlaceOrder RPC handler parses optional signal_id from proto request, returns error for malformed UUIDs
- Full signal_id passthrough from request to database to proto response enables signal-to-order audit chain
- RecordSignal endpoint marked deprecated with migration guidance comment

## Task Commits

Each task was committed atomically:

1. **Task 1: Add SignalID to OrderRecord and CreateOrderRequest** - `0270ef6` (feat)
2. **Task 2: Wire signal_id through PlaceOrder handler and convertOrder** - `1648bef` (feat)

## Files Created/Modified
- `src/go/backtester-api/models/order_record.go` - Added SignalID *uuid.UUID field with gorm:column,type,index tags
- `src/go/backtester-api/models/create_order_request.go` - Added SignalID *uuid.UUID field with json tag, added uuid import
- `src/go/backtester-api/router/grpc.go` - PlaceOrder parses signal_id, convertOrder maps to proto, RecordSignal deprecation comment
- `src/go/data/database_service.go` - commitOrderRecord sets order.SignalID = req.SignalID after PopulateOrderRecord

## Decisions Made
- SignalID set post-construction in commitOrderRecord rather than modifying PopulateOrderRecord's 20+ parameter signature (per research Pitfall 2)
- RecordSignal body left unchanged with deprecation comment only (per D-04 design decision)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Pre-existing test failures (TestSymbol, TestCalendar) due to missing .env file from external project and Postgres connection. These are not regressions from this plan's changes (verified by running tests on base commit).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 17 complete: TradeSignal types (plan 01) and signal_id order wiring (plan 02) both done
- Ready for Phase 18: Signal Repository & Sim Mode (ISignalRepository interface, in-memory implementation)
- signal_id is optional during migration period per D-03, will become required after Phase 22

---
*Phase: 17-signal-foundation*
*Completed: 2026-03-30*
