## ADDED Requirements

### Requirement: Headless server bootstrap
The smoke test SHALL boot the Go trading server in development mode (`GO_ENV=development`, matching `cmd/run-dev.sh`) as a background process, and SHALL wait for the Twirp RPC port (5051) to accept TCP connections before proceeding to any test phase, subject to a bounded startup timeout.

#### Scenario: Server becomes ready within the boot timeout
- **WHEN** the smoke test orchestration script starts the Go server via `go run ./main.go` with `GO_ENV=development`
- **THEN** the script SHALL detect Twirp port 5051 accepting connections within the configured boot timeout and proceed to the test phase

#### Scenario: Server fails to boot within the timeout
- **WHEN** Twirp port 5051 does not accept connections before the boot timeout elapses
- **THEN** the orchestration script SHALL terminate any partially-started server process and exit non-zero without running the test phase

### Requirement: REST port conflict tolerance
The smoke test SHALL treat a REST port (8080) bind failure as non-fatal — per the documented CLAUDE.md gotcha that the REST server logs an error but does not crash — and SHALL gate readiness exclusively on the Twirp port (5051).

#### Scenario: Port 8080 already in use
- **WHEN** another process already holds port 8080 at server boot time
- **THEN** the smoke test SHALL still pass, because it only waits for and depends on Twirp port 5051

### Requirement: Simulation playground creation over AAPL 2025 data
The smoke test SHALL create a Simulation-mode Playground via the `CreatePolygonPlaygroundRequest` Twirp RPC using the AAPL symbol and a fixed 2025 date range known to succeed against the Feed, and SHALL fail immediately if playground creation returns an error or an empty playground ID.

#### Scenario: Playground created successfully
- **WHEN** the smoke test issues `CreatePolygonPlaygroundRequest` for AAPL over the configured 2025 date range
- **THEN** it SHALL receive a non-empty Playground ID and an initial Account balance before continuing to the tick loop

#### Scenario: Playground creation fails
- **WHEN** `CreatePolygonPlaygroundRequest` returns an RPC error or an empty Playground ID
- **THEN** the smoke test SHALL exit non-zero immediately without entering the tick loop

### Requirement: Bounded tick loop
The smoke test SHALL run the Tick loop for at most a configured maximum number of ticks, or until the Playground's configured stop date is reached, whichever occurs first, and SHALL additionally enforce a wall-clock timeout so the test can never hang indefinitely when run as a recurring, unattended gate.

#### Scenario: Loop terminates on tick bound or stop date
- **WHEN** the smoke test's tick loop reaches either the configured max-tick count or the Playground's stop date
- **THEN** the loop SHALL exit cleanly without raising an exception

#### Scenario: Loop exceeds the wall-clock timeout
- **WHEN** neither the max-tick count nor the stop date is reached before the configured wall-clock timeout
- **THEN** the smoke test SHALL fail with a non-zero exit code and a message identifying the timeout

### Requirement: At least one order placed and filled
Within the bounded tick loop, the smoke test SHALL place at least one deterministic order against the Simulated Broker (independent of any options-strategy signal-generation timing) and SHALL assert that the order reaches a filled state before the loop's terminal condition.

#### Scenario: Demo order fills
- **WHEN** the smoke test's order is placed early in the tick loop
- **THEN** the smoke test SHALL observe that order's status transition to filled before the tick loop's terminal condition is reached

#### Scenario: No fill occurs
- **WHEN** no order reaches filled status by the time the tick loop's terminal condition (max ticks, stop date, or timeout) is reached
- **THEN** the smoke test SHALL fail with a non-zero exit code identifying the unfilled order

### Requirement: Clean teardown on pass and fail paths
The smoke test SHALL remove the Playground from the server and terminate the Go server process (and any child processes) on both the pass path and every failure path, so a run never leaves an orphaned `go run` process or an orphaned Playground.

#### Scenario: Teardown after a passing run
- **WHEN** the tick loop and all assertions complete successfully
- **THEN** the smoke test SHALL remove the Playground from the server and terminate the server process group before exiting 0

#### Scenario: Teardown after a failing run
- **WHEN** any assertion or bounded-loop condition fails
- **THEN** the smoke test SHALL still terminate the server process group before exiting non-zero, leaving no stray server process behind

### Requirement: Deterministic headless exit code
The smoke test SHALL be runnable non-interactively from a single command and SHALL exit 0 if and only if server boot, playground creation, the bounded tick loop, the order-fill assertion, and teardown all succeed, so it can serve as gate G5 for other OpenSpec changes and CI runs without a human watching the output.

#### Scenario: All checks pass
- **WHEN** server boot, playground creation, the bounded tick loop, the order-fill assertion, and teardown all succeed
- **THEN** the smoke test process SHALL exit with code 0

#### Scenario: Any check fails
- **WHEN** any of server boot, playground creation, the bounded tick loop, the order-fill assertion, or teardown raises an error
- **THEN** the smoke test process SHALL exit with a non-zero code

### Requirement: Taskfile entry point
The smoke test SHALL be invocable via a Taskfile target, `task test:smoke`, so operators and other automation (including future OpenSpec changes that depend on gate G5) can run the full headless flow without knowing the underlying script paths, per the project convention that every new operator-visible command gets a matching Taskfile target in the same change.

#### Scenario: Running via Task
- **WHEN** an operator or automation runs `task test:smoke` from the repo root
- **THEN** the full headless smoke test SHALL execute exactly as described in the requirements above, and Task's own exit code SHALL equal the smoke test's exit code
