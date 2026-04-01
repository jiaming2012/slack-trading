# Phase 20: ESDB Persistence - Discussion Log

> **Audit trail only.**

**Date:** 2026-03-31
**Phase:** 20-esdb-persistence
**Areas discussed:** Single stream confirmation, ESDB queryability

---

## Single Stream Confirmation

| Option | Description | Selected |
|--------|-------------|----------|
| Single stream | One 'trade-signals' stream, global ordering | ✓ |
| Per-symbol streams | Better read performance per symbol | |

**User's question:** "Why would per symbol streams give better performance?"
**Tradeoff explained:** Per-symbol = O(signals for symbol) reads, single = O(all signals) reads. At <1000 signals/day, negligible difference.
**User's choice:** Single stream confirmed

---

## ESDB Queryability

| Option | Description | Selected |
|--------|-------------|----------|
| Read stream + filter in Go | Deserialize and filter in application | ✓ |
| ESDB $by_category projections | Built-in but conflicts with single stream | |

**User's choice:** Read stream + filter in Go (Recommended)

---

## Claude's Discretion

- ESDBSignalRepository internal design
- Environment detection mechanism
- Integration test structure
- ESDB retention policy
