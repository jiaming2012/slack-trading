# Codebase Concerns

**Analysis Date:** 2026-03-25

## Tech Debt

**Duplicated Model Packages (`models` vs `eventmodels`):**
- Issue: Core domain types are duplicated across `src/go/models/` and `src/go/eventmodels/`. Files with identical or near-identical names exist in both: `price_level.go`, `strategy.go`, `trade.go`, `account.go`, `error.go`. The error files are nearly line-for-line copies with minor naming differences (e.g., `DuplicateCloseTradeErr` vs `ErrDuplicateCloseTrade`).
- Files: `src/go/models/error.go`, `src/go/eventmodels/error.go`, `src/go/models/price_level.go`, `src/go/eventmodels/price_level.go`, `src/go/models/strategy.go`, `src/go/eventmodels/strategy.go`, `src/go/models/trade.go`, `src/go/eventmodels/trade.go`, `src/go/models/account.go`, `src/go/eventmodels/account.go`
- Impact: Confusion about which package to import; risk of divergent behavior between copies; TODO comments in code acknowledge this (e.g., `src/go/eventmodels/instrument.go:9` "todo: refactor eventmodels and models to make class OrderRecordClass").
- Fix approach: Consolidate into a single domain model package. Remove the legacy `src/go/models/` package and migrate all references to `src/go/eventmodels/` or `src/go/backtester/models/`.

**Duplicated Generated Proto Stubs:**
- Issue: Identical generated protobuf/Twirp files exist in two locations.
- Files: `src/go/playground/playground.twirp.go` (6657 lines) and `src/go/backtester/playground/playground.twirp.go` (6657 lines). These are byte-identical.
- Impact: Double the generated code to maintain; risk of one copy going stale if `task gen:proto` only regenerates one.
- Fix approach: Remove one copy and update imports to use a single location.

**Deprecated Code Still in Repository:**
- Issue: A `deprecated/` directory with old Go and Python code remains in the repo.
- Files: `deprecated/go/cmd/sandbox.go`, `deprecated/go/cmd/stats/`, `deprecated/go/cmd/telemetry/`, `deprecated/python/`
- Impact: Increases repo size; may confuse contributors about what is current.
- Fix approach: Archive or delete. The `deprecated/README.md` should document why code was deprecated.

**Deprecated Functions Still Called:**
- Issue: `readStreamDeprecated` in `src/go/api/esdb_producer.go:233` is still actively called at line 385, despite being marked deprecated.
- Files: `src/go/api/esdb_producer.go`
- Impact: The deprecated path is the active code path, meaning the "replacement" was never completed.
- Fix approach: Complete the migration to the replacement implementation (referenced as `esdbConsumer` at line 232).

**Massive TODO Backlog (70+ items):**
- Issue: Over 70 TODO/FIXME comments scattered across Go and Python code, many referencing fundamental design issues.
- Files: Concentrated in `src/go/backtester/models/playground.go` (15+ TODOs), `src/go/api/esdb_producer.go` (6 TODOs), `src/go/eventmodels/` (10+ TODOs)
- Impact: Indicates accumulated shortcuts. Key examples:
  - `src/go/backtester/models/playground.go:2267`: "todo: fix - free margin should be calculated on each open order, not the total position" -- financial calculation bug
  - `src/go/backtester/models/playground.go:844`: "TODO: this should use the saga pattern" -- data consistency risk in reconciliation
  - `src/go/backtester/models/playground.go:1681`: "todo: make dbService non-nilable" -- nil checks scattered as workaround
- Fix approach: Triage TODOs by severity. The free margin calculation bug (line 2267) and saga pattern needs (lines 844, 917) should be prioritized.

**Vendored Python Environment Committed:**
- Issue: A full Python virtualenv is committed under `src/cmd/stats/env/` with thousands of library files (plotly, pandas, scipy, numpy, etc.).
- Files: `src/cmd/stats/env/lib/python3.10/site-packages/` (the largest files in the repo by line count)
- Impact: Massively inflates repository size. Git operations are slower.
- Fix approach: Add `src/cmd/stats/env/` to `.gitignore` and remove from tracking. Use `requirements.txt` or conda env specification instead.

## Known Bugs

**Free Margin Calculation Error:**
- Symptoms: Free margin is calculated on total position rather than per open order, leading to incorrect margin availability.
- Files: `src/go/backtester/models/playground.go:2267`
- Trigger: When multiple open orders exist simultaneously.
- Workaround: None documented.

**Intentionally Failing Test:**
- Symptoms: `require.Fail(t, "finish the test")` causes test failures.
- Files: `src/go/marketdata/integration_tests/polygon_client_test.go:46`
- Trigger: Running integration tests.
- Workaround: Documented in CLAUDE.md as intentional stub.

**OrderRecord Status Mismatch:**
- Symptoms: Order status returns `OrderRecordStatusNew` when it should return `pending`.
- Files: `src/go/backtester/models/order_record.go:536`, `src/go/backtester/models/playground_test.go:4313`
- Trigger: New orders created before being processed.
- Workaround: Tests assert current (incorrect) behavior.

## Security Considerations

**Hardcoded Coinbase WebSocket Secret:**
- Risk: A Coinbase API secret is hardcoded as a string literal directly in source code.
- Files: `src/go/worker/websockets.go:125` -- `const secret = "s2RceoHWEaLYxnaeOUm2tpmNLsELkaGy"`
- Current mitigation: None. The secret is committed to git history.
- Recommendations: Immediately rotate this secret. Move to environment variable. Add a pre-commit hook to scan for hardcoded secrets.

**Excessive Use of `log.Fatalf` and `panic`:**
- Risk: 38 occurrences of `log.Fatal`/`os.Exit` and multiple `panic()` calls across 24 files. In a production server, these cause unclean shutdowns, potentially leaving database transactions uncommitted or broker orders in inconsistent states.
- Files: `src/go/api/esdb_producer.go` (5 occurrences), `src/go/api/interactive_brokers.go` (3 occurrences), `src/go/backtester/services/order_queue.go` (2 occurrences), `src/go/data/database_service.go:842`, `src/go/marketdata/polygon.go:601`, `src/go/workers/tradier_api_worker.go:499`
- Current mitigation: None.
- Recommendations: Replace `log.Fatalf` with proper error returns. Use graceful shutdown patterns. Reserve `panic` only for truly unrecoverable startup errors, never in request handlers or event loops.

**Database Credentials in Code Comments:**
- Risk: Default database credentials (`grodt`/`test747`) are documented in memory files, potentially making them discoverable.
- Files: Referenced in project memory/documentation.
- Current mitigation: These appear to be development-only credentials.
- Recommendations: Ensure production credentials are never in code or documentation.

## Performance Bottlenecks

**Excessive Mutex Usage Without Clear Hierarchy:**
- Problem: 20+ mutex instances across the codebase with no documented locking order.
- Files: `src/go/data/database_service.go:59,70` (2 mutexes), `src/go/models/strategy.go:22-23` (2 mutexes per strategy), `src/go/models/account.go:20`, `src/go/models/price_level.go:45`, `src/go/marketdata/polygon_cache.go:29,35,41` (3 RWMutexes), `src/go/workers/tracker_consumer_v3.go:22`
- Cause: Organic growth without a concurrency design. Some structs have multiple mutexes protecting different fields.
- Improvement path: Document lock ordering to prevent deadlocks. Consider reducing granularity or using channels where appropriate.

**PL() Calculation Overhead:**
- Problem: A TODO at `src/go/eventmodels/account.go:143` notes: "todo: analyze if calling PL() so many times on each tick causes a bottleneck."
- Files: `src/go/eventmodels/account.go:143`
- Cause: PL (profit/loss) is recalculated on every tick for every open position.
- Improvement path: Cache PL values and invalidate only when positions change.

**Missing Candle Cache:**
- Problem: Two TODO comments indicate candle data is not cached.
- Files: `src/go/marketdata/utils.go:31`, `src/go/backtester/router/service.go:32`
- Cause: Candle data is re-fetched on each request.
- Improvement path: Implement caching similar to the existing `polygon_cache.go` pattern.

**Backtester Performance TODOs (Phases 3-5):**
- Problem: Three documented performance improvements are unimplemented.
- Files: `src/go/marketdata/polygon_cache.go:24-26`, `src/go/backtester/router/grpc.go:213`, `src/clients/python/backtester_playground_client_grpc.py:544`
- Cause: Each Python tick requires a separate RPC round-trip; account state is fetched per-tick; option chain data is fetched lazily.
- Improvement path: Phase 3 (embed account state in TickDelta), Phase 4 (BatchTick RPC), Phase 5 (pre-fetch option chains).

## Fragile Areas

**`src/go/backtester/models/playground.go` (2909 lines):**
- Files: `src/go/backtester/models/playground.go`
- Why fragile: God object combining order management, tick simulation, reconciliation, account management, and database persistence. Contains 15+ TODOs acknowledging design issues. Two separate reconciliation blocks (lines 844, 917) both need saga pattern but use direct mutation.
- Safe modification: Changes to this file should be accompanied by tests in `playground_test.go` (4544 lines). Be aware of the nil dbService workaround (lines 1681, 1698).
- Test coverage: Extensive test file exists but tests assert some known-incorrect behaviors (e.g., status should be "pending" not "new").

**`src/go/backtester/router/grpc.go` (1266 lines):**
- Files: `src/go/backtester/router/grpc.go`
- Why fragile: Main RPC handler with two `panic("not implemented")` calls (lines 201, 622) that would crash the server if hit. Mixes request parsing, business logic, and response formatting.
- Safe modification: Test through the Twirp client. Avoid triggering the unimplemented code paths.
- Test coverage: No dedicated test file for grpc.go handlers.

**`src/go/data/database_service.go` (1634 lines):**
- Files: `src/go/data/database_service.go`
- Why fragile: Contains a `log.Fatalf` at line 842 that will crash the server if saving a live repository fails. Two separate mutexes with unclear interaction. Commented-out code and TODO items.
- Safe modification: Wrap database operations in transactions. Replace `log.Fatalf` with error returns.
- Test coverage: No dedicated unit test file found.

**`src/clients/python/credit_spread_strategy.py` (1964 lines) and `options_strategy_basic_v7.py` (1650 lines):**
- Files: `src/clients/python/credit_spread_strategy.py`, `src/clients/python/options_strategy_basic_v7.py`
- Why fragile: Large monolithic strategy files. The "v7" suffix suggests multiple prior iterations without cleanup.
- Safe modification: Extract shared logic into utility modules. Strategy files should focus on signal logic only.
- Test coverage: `test_demo_covered_call.py` covers options_strategy_basic_v7 via integration test, but no unit tests for internal functions.

**Order Reconciliation Without Saga Pattern:**
- Files: `src/go/backtester/models/playground.go:844`, `src/go/backtester/models/playground.go:917`
- Why fragile: Two blocks of reconciliation logic mutate multiple orders without transactional guarantees. TODO comments explicitly call for saga pattern (e.g., Temporal).
- Safe modification: Wrap in database transactions at minimum. Test reconciliation scenarios end-to-end.
- Test coverage: Gap in testing partial failure scenarios during reconciliation.

## Scaling Limits

**Single-Process Event Bus:**
- Current capacity: In-process pub/sub via `src/go/pubsub/`.
- Limit: Cannot scale horizontally; all event processing happens in a single Go process.
- Scaling path: Migrate to external message broker (NATS, Kafka) or use EventStoreDB subscriptions more extensively.

**Per-Tick RPC Round Trips:**
- Current capacity: Each backtest tick requires Python-to-Go RPC call(s).
- Limit: Backtest speed is bounded by network latency and serialization overhead. A 1-year backtest with minute candles requires ~100K+ RPC calls.
- Scaling path: Implement BatchTick RPC (Phase 4 TODO) and embed account state in responses (Phase 3 TODO).

## Dependencies at Risk

**numpy Pinned to 1.26.4:**
- Risk: pandas_ta requires numpy < 2.0, creating a version ceiling. numpy 1.x is approaching end of maintenance.
- Impact: Cannot adopt newer numpy features; eventual incompatibility with other packages requiring numpy >= 2.0.
- Migration plan: Replace pandas_ta with a maintained alternative or fork it to support numpy 2.x.

**Go 1.22.4 (Not Latest):**
- Risk: Go 1.22 is current but will eventually lose support. The `.4` patch version suggests this was pinned at a specific point.
- Impact: Low immediate risk. Monitor for security patches.
- Migration plan: Update `go.mod` when ready; test with `go vet` and full test suite.

**EventStoreDB Client v4:**
- Risk: `github.com/EventStore/EventStore-Client-Go/v4` -- EventStoreDB Go client ecosystem is relatively niche.
- Impact: Limited community support for debugging; potential breaking changes in future versions.
- Migration plan: Monitor for v5+ releases. Consider whether EventStoreDB is still the right event store.

## Missing Critical Features

**No Graceful Shutdown:**
- Problem: `log.Fatalf` calls throughout the codebase indicate no graceful shutdown handling. Server crashes leave state inconsistent.
- Blocks: Production reliability. Broker orders may be left in unknown states.

**No Retry Logic for External API Calls:**
- Problem: Polygon, Tradier, and other external API calls lack retry/backoff logic.
- Files: `src/go/marketdata/polygon.go`, `src/go/backtester/services/tradier_broker.go`
- Blocks: Reliability under network instability. Polygon 403 errors (documented in CLAUDE.md) are handled as hard failures.

**Missing Database Transaction Boundaries:**
- Problem: Multiple TODO comments reference the need for transactions (e.g., `src/go/backtester/models/playground.go:2417,2423` "todo: place all changes inside of a single transaction").
- Blocks: Data consistency during multi-step operations like order reconciliation.

## Test Coverage Gaps

**No Unit Tests for RPC Handlers:**
- What's not tested: The main API surface in `src/go/backtester/router/grpc.go` (1266 lines) has no dedicated test file.
- Files: `src/go/backtester/router/grpc.go`
- Risk: RPC handler logic changes can break the Python client contract without detection.
- Priority: High

**No Unit Tests for Database Service:**
- What's not tested: `src/go/data/database_service.go` (1634 lines) has no unit test file. Only covered indirectly via E2E tests.
- Files: `src/go/data/database_service.go`
- Risk: Database query changes can silently break data loading/saving.
- Priority: High

**Incomplete Integration Tests:**
- What's not tested: `src/go/marketdata/integration_tests/polygon_client_test.go` contains `require.Fail` stub.
- Files: `src/go/marketdata/integration_tests/polygon_client_test.go:46`
- Risk: Polygon API integration is not validated by CI.
- Priority: Medium

**Python Strategy Unit Tests Missing:**
- What's not tested: Internal functions within strategy files (`options_strategy_basic_v7.py`, `credit_spread_strategy.py`, `mean_reversion_strategy.py`). Only integration-level tests exist (`test_demo_covered_call.py`, `test_credit_spread_strategy.py`).
- Files: `src/clients/python/options_strategy_basic_v7.py`, `src/clients/python/credit_spread_strategy.py`
- Risk: Strategy logic bugs (signal generation, position sizing) go undetected until full backtest runs.
- Priority: Medium

---

*Concerns audit: 2026-03-25*
