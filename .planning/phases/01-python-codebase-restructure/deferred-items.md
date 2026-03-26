# Deferred Items - Phase 01

## Pre-existing Test Failures in test_credit_spread_strategy.py

20 test failures in `src/clients/python/tests/test_credit_spread_strategy.py` that existed before the Phase 01 restructure. These are NOT caused by the restructure and are out of scope for Phase 01.

**Affected test classes:**
- `TestSkipFilters` (multiple tests: min_credit_filter, debit_spread_rejected, negative_ev_skipped, wide_bid_ask_skipped, duplicate_strike_expiry_skipped, and others)
- `TestProbabilityAndEV` (test_expected_profit_all_win, test_expected_profit_all_max_loss, test_expected_profit_mixed, test_expected_profit_partial_loss_math)
- Various other test classes with pre-existing logic mismatches

**Discovered during:** Plan 01-04, Task 2
**Status:** Out of scope - pre-existing failures unrelated to the restructure
