---
phase: 2
slug: go-otel-foundation-local-backend
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-26
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test + manual verification |
| **Config file** | none |
| **Quick run command** | `go test -run "TestSetupOTelSDK\|TestOTelShutdown\|TestLogfmtFormat" -count=1 ./src/go/utils/...` |
| **Full suite command** | `go test -count=1 ./src/go/utils/... && go build ./cmd/main.go && docker compose -f observability/docker-compose.yaml config --quiet` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run quick command
- **After every plan wave:** Run full suite command
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| P01-T1 | 01 | 1 | OTEL-01,02,03,05 | unit | `go test -run "TestSetupOTelSDK\|TestOTelShutdown\|TestLogfmtFormat" -count=1 ./src/go/utils/...` | Created by tdd task | pending |
| P01-T2 | 01 | 1 | OTEL-04,06 | build | `go build ./cmd/main.go` | Existing | pending |
| P02-T1 | 02 | 1 | BACK-01 | infra | `docker compose -f observability/docker-compose.yaml config --quiet` | Created by task | pending |
| P02-T2 | 02 | 1 | BACK-02,03 | smoke | `curl -sf http://localhost:3000/api/health && curl -sf http://localhost:3000/api/datasources` | Manual + automated | pending |
| TBD | TBD | TBD | OTEL-04 | integration | Start server, make RPC call, check Grafana/Tempo | Manual | pending |

*Status: pending -- green -- red -- flaky*

---

## Nyquist Compliance

Plan 01 Task 1 uses `tdd="true"`, which means the test file (`otel_test.go`) is written RED before implementation goes GREEN -- this is equivalent to a Wave 0 test scaffold. The TDD task creates its own test infrastructure inline, satisfying the Nyquist requirement that every verify has an automated command and test files exist before implementation verification runs.

Plan 02 Task 1 creates infrastructure (Docker Compose) verified by `docker compose config`. Task 2 (checkpoint) uses a retry-until-ready loop (not sleep) to verify Grafana health and datasource availability.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Traces visible in Grafana/Tempo | OTEL-04 | Requires running server + Docker + RPC call | Start otel-lgtm, start server, curl Twirp endpoint, open Grafana Tempo explorer |
| Grafana data sources pre-configured | BACK-03 | Visual confirmation desired | Open localhost:3000, check Connections > Data Sources |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (TDD task creates tests inline)
- [x] No watch-mode flags
- [x] Feedback latency < 10s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** ready
