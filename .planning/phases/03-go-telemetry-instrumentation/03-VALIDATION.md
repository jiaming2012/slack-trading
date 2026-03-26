---
phase: 3
slug: go-telemetry-instrumentation
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-26
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none |
| **Quick run command** | `go test -count=1 ./src/go/telemetry/... ./src/go/backtester-api/...` |
| **Full suite command** | `go test -count=1 ./src/go/... && go build ./cmd/main.go` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run quick command
- **After every plan wave:** Run full suite command
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | Wave 0 Status | Status |
|---------|------|------|-------------|-----------|-------------------|---------------|--------|
| P01-T1 | 01 | 1 | ORD-04 | unit | `go test -run TestShouldEmitOrderTelemetry -count=1 ./src/go/telemetry/...` | TDD task (tdd="true") — tests created inline RED before GREEN | pending |
| P01-T2 | 01 | 1 | ORD-01,02,03 | build+existing | `go build ./cmd/main.go && go test -count=1 ./src/go/backtester-api/...` | N/A — instrumentation task, verified by build + existing tests | pending |
| P02-T1 | 02 | 2 | DATA-01,03 | build+existing | `go build ./src/go/backtester-api/... && go test -count=1 ./src/go/backtester-api/...` | N/A — instrumentation task, verified by build + existing tests | pending |
| P02-T2 | 02 | 2 | DATA-02 | build+existing | `go build ./src/go/backtester-api/... && go test -count=1 ./src/go/backtester-api/...` | N/A — instrumentation task, verified by build + existing tests | pending |
| P02-T3 | 02 | 2 | D-10 | build | `go build ./src/go/eventconsumers/...` | N/A — single counter call, verified by build | pending |
| P03-T1 | 03 | 2 | BEAT-01,02 | unit | `go test -run TestHeartbeat -count=1 ./src/go/telemetry/...` | TDD task (tdd="true") — tests created inline RED before GREEN | pending |
| P03-T2 | 03 | 2 | BEAT-01 | build | `go build ./cmd/main.go && go build ./src/go/...` | N/A — wiring task, verified by build | pending |

*Status: pending — green — red — flaky*

---

## Wave 0 Requirements

TDD tasks (Plan 01 Task 1 and Plan 03 Task 1) have `tdd="true"` with explicit `<behavior>` blocks. Tests are created inline during execution following the RED-GREEN-REFACTOR cycle — the test file is written first (RED), then implementation (GREEN). This is the Wave 0 equivalent: no separate Wave 0 plan is needed because TDD tasks create their own test scaffolds before implementation.

Non-TDD tasks (instrumentation/wiring) are verified by `go build` compilation + existing test suites. These tasks add structured log calls and counter increments to existing code — they don't introduce new testable behavior that requires dedicated test files.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Heartbeat visible in Grafana | BEAT-01 | Requires running server + observability stack | Start otel-lgtm, start server, check Grafana metrics explorer for heartbeat gauge |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 / TDD dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covered by TDD tasks with `<behavior>` blocks (RED before GREEN)
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter
- [x] `wave_0_complete: true` set in frontmatter

**Approval:** approved
