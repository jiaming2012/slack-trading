---
phase: 1
slug: python-codebase-restructure
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-26
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | pytest (conda env grodt) |
| **Config file** | none — tests run via pytest directly |
| **Quick run command** | `cd src/clients/python && /Users/jamal/miniconda3/envs/grodt/bin/python -m pytest tests/ -x -q --tb=short 2>&1 | head -50` |
| **Full suite command** | `cd src/clients/python && /Users/jamal/miniconda3/envs/grodt/bin/python -m pytest tests/ -v` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run quick command
- **After every plan wave:** Run full suite command
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | DIR-01..08 | integration | `python -c "from strategies import *; from engine import *; from lib import *"` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | CONS-01 | regression | `pytest tests/test_demo_covered_call.py -v` | ✅ | ⬜ pending |
| TBD | TBD | TBD | CONS-01 | regression | `pytest tests/test_credit_spread_strategy.py -v` | ✅ | ⬜ pending |
| TBD | TBD | TBD | CONS-01 | regression | `pytest tests/test_mean_reversion_strategy.py -v` | ✅ | ⬜ pending |
| TBD | TBD | TBD | CONS-04 | regression | `pytest tests/ -v` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `src/clients/python/strategies/__init__.py` — package init for strategies
- [ ] `src/clients/python/engine/__init__.py` — package init for engine
- [ ] `src/clients/python/lib/__init__.py` — package init for lib
- [ ] `src/clients/python/tools/__init__.py` — package init for tools
- [ ] `src/clients/python/demos/__init__.py` — package init for demos
- [ ] `src/clients/python/tests/__init__.py` — package init for tests

*Existing test infrastructure (pytest) covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Demo scripts run end-to-end | CONS-03 | Requires running Go server | Start server with `task app:dev`, then run `python -m demos.demo_covered_call --symbol AAPL --start 2024-06-01 --end 2024-12-31` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
