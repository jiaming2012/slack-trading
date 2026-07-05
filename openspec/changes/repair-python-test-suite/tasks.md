# Tasks — repair-python-test-suite

## 1. Triage and fixture contract

- [ ] 1.1 Empirically derive the playground-accessor contract: instrument construction + one tick of the three strategies (`deprecated.mean_reversion`, `deprecated.options_mean_reversion`, `deprecated.credit_spread`) against a spy object; commit the accessor list as the fixture's contract doc.
- [ ] 1.2 Build the shared fake playground fixture (`tests/fixtures/mock_playground.py` + conftest wiring): typed defaults, per-test overrides, AttributeError on unknown accessors (no silent MagicMock leakage).

## 2. Bucket A — strategy tests (147)

- [ ] 2.1 Adopt the fixture in `tests/test_mean_reversion_strategy.py`; file green (49/49 of its baseline IDs pass).
- [ ] 2.2 Adopt the fixture in `tests/test_options_mean_reversion_strategy.py`; file green (54).
- [ ] 2.3 Adopt the fixture in `tests/test_credit_spread_strategy.py`; file green (44).
- [ ] 2.4 Confirm no assertion was weakened: diff review of assertion lines vs pre-change (setup-only changes expected).

## 3. Bucket B — engine interface doubles (~14)

- [ ] 3.1 Update `MockStrategy` (and any sibling doubles) to the full current strategy interface incl. `on_signal`; `tests/test_trading_engine_otel.py` green (10).
- [ ] 3.2 `tests/test_heartbeat.py` and `tests/test_trace_propagation.py` green (4).

## 4. Bucket C — persistence fixtures (4)

- [ ] 4.1 Fix stats-row fixtures to the 4-column shape; `tests/test_persistence.py` green.

## 5. Bucket D — per-case triage (~14)

- [ ] 5.1 `tests/test_demo_covered_call.py` (8): diagnose per design.md rules; re-pin only with the causing commit named, else STOP and report.
- [ ] 5.2 `tests/test_mean_reversion_diff.py` (2), `tests/test_otel.py` (2), `tests/test_client_trace_id.py` (2): diagnose and fix per-case; STOP on any suspected real product regression.

## 6. Gate retirement and entry point

- [ ] 6.1 Add `task test:python` (headless, standing exclusions, propagates pytest exit code).
- [ ] 6.2 Delete `tests/baselines/pytest-failures-20260704.txt`; update HANDOFF.md G3-discipline gotcha to the zero-failure rule.

## 7. Verification and closeout

- [ ] 7.1 `task test:python` green twice consecutively (zero failed; only harness-guard skips).
- [ ] 7.2 G1 `go build ./src/go/... ./cmd/...` and G2 `task test` green (untouched).
- [ ] 7.3 `task test:smoke` green once.
- [ ] 7.4 G6 — Fable adversarial review approves (test-honesty focus: assertions not weakened to force green).
- [ ] 7.5 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time).
