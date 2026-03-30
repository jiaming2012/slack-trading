---
phase: 17-signal-foundation
plan: 01
subsystem: api
tags: [protobuf, eventmodels, trade-signal, esdb, twirp]

# Dependency graph
requires: []
provides:
  - "TradeSignal Go struct with SavedEvent implementation in eventmodels"
  - "SignalName typed string registry with 4 constants and Validate()"
  - "TradeSignalProto protobuf message with map<string,string> attributes"
  - "signal_id optional field on PlaceOrderRequest and Order proto messages"
  - "TradeSignalStream constant and NewTradeSignalStreamName per-symbol function"
  - "TradeSignalEventName event constant"
affects: [17-02, 18-signal-persistence, 19-signal-rpc, 21-migration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SavedEvent pattern: embed BaseRequestEvent, implement GetSavedEventParameters with per-symbol stream"
    - "SignalName typed string with Validate() switch for extensible signal registry"

key-files:
  created:
    - "src/eventmodels/trade_signal.go"
    - "src/eventmodels/trade_signal_test.go"
  modified:
    - "src/eventmodels/signal_name.go"
    - "src/eventmodels/stream_names.go"
    - "src/eventmodels/event_names.go"
    - "src/playground.proto"
    - "src/playground/playground.pb.go"
    - "src/playground/playground.twirp.go"
    - "src/cmd/stats/rpc/playground_pb2.py"
    - "src/cmd/stats/rpc/playground_twirp.py"

key-decisions:
  - "Preserved existing SignalName constants (SuperTrend*, StochasticRsi*) and appended new ones"
  - "Used plan-specified field numbers (17, 26) for signal_id even though not contiguous with existing fields, for forward compatibility"
  - "Skipped RecordSignalRequest deprecation comment since that message does not exist in current proto"
  - "Attributes is map[string]interface{} in Go struct, map<string,string> in proto per D-02 design decision"

patterns-established:
  - "TradeSignal SavedEvent pattern: per-symbol ESDB stream naming (trade-signals-AAPL)"
  - "SignalName registry: typed string constants with Validate() for extensibility"

requirements-completed: [SIG-01, SIG-03]

# Metrics
duration: 3min
completed: 2026-03-30
---

# Phase 17 Plan 01: Signal Foundation Summary

**TradeSignal Go struct with UUID, SignalName registry, SavedEvent implementation, and TradeSignalProto proto message with signal_id on PlaceOrderRequest and Order**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-30T20:17:30Z
- **Completed:** 2026-03-30T20:20:57Z
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments
- Created TradeSignal struct implementing SavedEvent interface with per-symbol stream naming
- Created SignalName typed string registry with 4 constants and validation
- Added TradeSignalProto proto message and signal_id fields on PlaceOrderRequest (field 17) and Order (field 26)
- All 5 unit tests passing (constructor, SavedEvent params, validation known/unknown, JSON round-trip)
- Full Go build passes with regenerated proto stubs

## Task Commits

Each task was committed atomically:

1. **Task 1: Create SignalName registry and TradeSignal struct (RED)** - `7de6c1f` (test)
2. **Task 1: Create SignalName registry and TradeSignal struct (GREEN)** - `04d8ba7` (feat)
3. **Task 2: Add TradeSignalProto to proto and regenerate stubs** - `8591f07` (feat)

_Note: Task 1 followed TDD with RED then GREEN commits_

## Files Created/Modified
- `src/eventmodels/trade_signal.go` - TradeSignal struct with SavedEvent implementation, NewTradeSignal constructor
- `src/eventmodels/trade_signal_test.go` - 5 unit tests covering constructor, params, validation, JSON round-trip
- `src/eventmodels/signal_name.go` - Added 4 new SignalName constants and Validate() method
- `src/eventmodels/stream_names.go` - Added TradeSignalStream constant and NewTradeSignalStreamName function
- `src/eventmodels/event_names.go` - Added TradeSignalEventName constant
- `src/playground.proto` - Added TradeSignalProto message, signal_id on PlaceOrderRequest and Order
- `src/playground/playground.pb.go` - Regenerated Go protobuf stubs
- `src/playground/playground.twirp.go` - Regenerated Go Twirp stubs
- `src/cmd/stats/rpc/playground_pb2.py` - Regenerated Python protobuf stubs
- `src/cmd/stats/rpc/playground_twirp.py` - Regenerated Python Twirp stubs

## Decisions Made
- Preserved existing SignalName constants (SuperTrend variants, StochasticRsi) rather than replacing them
- Used non-contiguous field numbers (17, 26) as specified in plan for proto forward compatibility
- Attributes type: map[string]interface{} in Go for flexibility, map<string,string> in proto for wire format simplicity

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] signal_name.go already existed with different content**
- **Found during:** Task 1
- **Issue:** Plan assumed signal_name.go needed to be created; it already existed with SuperTrend/StochasticRsi constants
- **Fix:** Appended new constants and Validate() method to existing file instead of overwriting
- **Files modified:** src/eventmodels/signal_name.go
- **Verification:** go build passes, all tests pass
- **Committed in:** 04d8ba7

**2. [Rule 3 - Blocking] RecordSignalRequest does not exist in proto**
- **Found during:** Task 2
- **Issue:** Plan specified adding deprecation comment to RecordSignalRequest but that message does not exist
- **Fix:** Skipped this step (no-op)
- **Verification:** N/A - no changes needed

**3. [Rule 3 - Blocking] Python stubs path differs from plan**
- **Found during:** Task 2
- **Issue:** Plan referenced src/clients/python/rpc/ but actual path is src/cmd/stats/rpc/
- **Fix:** Used correct path for Python stub generation
- **Files modified:** src/cmd/stats/rpc/playground_pb2.py, src/cmd/stats/rpc/playground_twirp.py
- **Verification:** Files generated successfully
- **Committed in:** 8591f07

---

**Total deviations:** 3 auto-fixed (3 blocking)
**Impact on plan:** All auto-fixes addressed path/file discrepancies between plan and actual codebase. No scope creep.

## Issues Encountered
None beyond the deviations noted above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- TradeSignal type and proto definition complete, ready for Phase 17 Plan 02 (if any additional foundation work)
- signal_id fields available for downstream phases to wire into order flow
- SavedEvent implementation ready for ESDB persistence in Phase 18/19

## Self-Check: PASSED

All 10 created/modified files verified present. All 3 task commits verified in git log.

---
*Phase: 17-signal-foundation*
*Completed: 2026-03-30*
