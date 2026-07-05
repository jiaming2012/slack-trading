## 1. Smoke strategy driver (Python)

- [x] 1.1 Write `src/clients/python/tests/e2e_smoke_strategy.py`: places one deterministic market buy order for the configured underlying stock shortly after playground creation, independent of any options-strategy signal logic
- [x] 1.2 Write `src/clients/python/tests/test_e2e_smoke.py`: creates the Simulation Playground for AAPL over a short fixed 2025 date window via `CreatePolygonPlaygroundRequest`, drives the bounded tick loop through the smoke strategy, and asserts the order reaches filled status before the loop's terminal condition
- [x] 1.3 Add the wall-clock timeout and max-tick-count bound to the tick loop, with a clear failure message identifying which bound was hit if neither terminal condition (max ticks / stop date) is reached in time
- [x] 1.4 Add teardown logic (remove Playground from the server) that runs on both the pass path and every failure path of `test_e2e_smoke.py`

## 2. Headless orchestration script

- [x] 2.1 Write `src/clients/python/run_e2e_smoke_test.sh`: start `go run ./main.go` from `cmd/` with `GO_ENV=development` (mirroring `cmd/run-dev.sh`'s env resolution for `OPTIONS_CONFIG_PATH`) as a background process, redirecting server output to a log file and capturing its PID/process group
- [x] 2.2 Add a bounded readiness poll on Twirp port 5051 only (explicitly ignoring REST port 8080 bind failures, per the CLAUDE.md port-8080-conflict gotcha); fail fast and terminate the partially-started server if the boot timeout elapses
- [x] 2.3 Invoke `test_e2e_smoke.py` against the ready server and capture its exit code
- [x] 2.4 Add teardown (SIGTERM, then SIGKILL after a grace period, targeting the server's process group) that always runs regardless of the test's exit code
- [x] 2.5 Propagate the captured pytest exit code as the script's own exit code (0 pass / nonzero fail)

## 3. Taskfile wiring

- [x] 3.1 Add a `test:smoke` target to `taskfile.yml` (`dir: src/clients/python`) invoking `./run_e2e_smoke_test.sh`
- [x] 3.2 Confirm `task test:smoke` runs end to end from a clean checkout with no manual setup beyond the existing `.env` / conda `grodt` environment prerequisites already documented in CLAUDE.md

## 4. Verification and closeout

- [x] 4.1 G1: run `go build ./src/go/... ./cmd/...` and confirm it is green
- [x] 4.2 G5: run `task test:smoke` headlessly end to end; confirm exit code 0, a non-empty Playground ID, at least one order transitioning to filled within the bounded tick loop, and a clean teardown log entry (no orphaned `go run` process left behind)
- [ ] 4.3 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time)
