# python-test-suite-health Specification

## Purpose
TBD - created by archiving change repair-python-test-suite. Update Purpose after archive.
## Requirements
### Requirement: Python suite runs green with zero failures

The Python client test suite (`pytest tests/` with the standing exclusions: `test_strategy_e2e.py`, and the e2e smoke module outside its harness) SHALL pass with zero failed tests on a machine with the `grodt` conda environment and no running server.

#### Scenario: Full suite green

- **WHEN** `OTEL_SDK_DISABLED=true python -m pytest tests/ --ignore=tests/test_strategy_e2e.py` runs from `src/clients/python` with no server on :5051
- **THEN** the run reports 0 failed (skips are permitted only for tests whose declared harness is absent, e.g. the e2e smoke module)

### Requirement: Shared mock playground fixture matches the live client contract

A single shared mock-playground fixture SHALL be provided for strategy tests, stubbing every accessor the strategy constructors and tick paths read (including current price, candles, account state, and position queries), with correctly-typed return values, so that no strategy test fails from an unstubbed `MagicMock` leaking into typed expressions.

#### Scenario: Strategy constructors succeed against the fixture

- **WHEN** each strategy under test (`deprecated.mean_reversion`, `deprecated.options_mean_reversion`, `deprecated.credit_spread`) is constructed against the shared fixture
- **THEN** construction completes without `TypeError`/`AttributeError` arising from mock leakage

#### Scenario: Fixture drift is detectable

- **WHEN** a strategy or engine change adds a new required playground accessor that the fixture does not stub
- **THEN** the affected tests fail loudly at construction or first use (no silent MagicMock arithmetic), keeping the fixture honest over time

### Requirement: Test doubles track the current strategy interface

Engine-facing test doubles (e.g. the otel/heartbeat/trace tests' MockStrategy) SHALL implement the full strategy interface the trading engine requires, including `on_signal`, so interface growth breaks doubles at definition rather than mid-test.

#### Scenario: Engine lifecycle tests pass with the updated double

- **WHEN** the trading-engine otel, heartbeat, and trace-propagation tests run
- **THEN** they pass, and the double exposes every attribute `engine/trading_engine.py` wires (`on_signal` included)

### Requirement: Persistence fixtures match the current stats row shape

Persistence tests SHALL feed the 4-column stats row (`total_trades, total_pnl, win_rate, profit_factor`) the production unpack expects.

#### Scenario: Persistence tests pass with 4-column fixtures

- **WHEN** `tests/test_persistence.py` runs
- **THEN** it passes with no `ValueError` from the stats unpack

### Requirement: Reference-metric re-pins are explained, never silent

Any change to a pinned reference metric (e.g. `test_demo_covered_call.py` expected values) SHALL be accompanied by a committed explanation identifying the commit or behavior change that legitimately moved the metric; re-pinning without a traced cause SHALL NOT occur.

#### Scenario: A drifted pin is re-baselined with cause

- **WHEN** a pinned metric no longer matches and the drift is traced to an intentional, already-shipped behavior change
- **THEN** the new pin lands together with a written explanation naming that cause

#### Scenario: An untraced drift blocks

- **WHEN** a pinned metric no longer matches and no legitimate cause can be identified
- **THEN** the test remains failing and the change STOPS for investigation rather than re-pinning

### Requirement: The parity baseline is retired for a zero-failure gate

Once the suite is green, the committed 179-ID baseline file SHALL be deleted and the G3 discipline documentation updated so future changes gate on "zero Python test failures" rather than "no new failures vs baseline".

#### Scenario: Baseline file removed

- **WHEN** the change completes
- **THEN** `src/clients/python/tests/baselines/pytest-failures-20260704.txt` no longer exists and HANDOFF.md documents the zero-failure gate

### Requirement: Operator entry point for the Python suite

A `task test:python` Taskfile target SHALL run the suite headlessly with the standard exclusions and propagate pytest's exit code.

#### Scenario: Task target runs the suite

- **WHEN** the operator runs `task test:python`
- **THEN** the full suite executes with the standing exclusions and the command exits 0 on green, nonzero otherwise

