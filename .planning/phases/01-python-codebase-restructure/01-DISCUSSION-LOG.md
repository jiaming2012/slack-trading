# Phase 1: Python Codebase Restructure - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-26
**Phase:** 01-python-codebase-restructure
**Areas discussed:** Strategy interface design, Trading engine adaptation

---

## Strategy Interface Design

### How unified should the strategy interface be?

| Option | Description | Selected |
|--------|-------------|----------|
| Thin adapter | Keep each strategy's internal logic as-is. Add wrapper for common contract. | |
| Full standardization | Common base class with standardized methods (init, tick, get_signals, is_complete). | ✓ |
| Protocol-based | Python Protocol for duck typing with type checking. | |

**User's choice:** Full standardization
**Notes:** User asked for trade-off analysis on readability and testability. Full standardization won on both: same flow across all 6 strategies (readable), same test harness for all (testable).

### Signal type consolidation

| Option | Description | Selected |
|--------|-------------|----------|
| Single OpenSignal + CloseSignal | Unify all versioned signals into one pair with optional fields | |
| Keep versioned signals | Keep existing types, return through common interface | |
| You decide | Claude picks cleanest approach | ✓ |

**User's choice:** You decide

---

## Trading Engine Adaptation

### Who controls the tick period?

| Option | Description | Selected |
|--------|-------------|----------|
| Strategy decides | Strategy returns next_tick_seconds — enables smart HTF/LTF optimization | ✓ |
| Engine decides | Fixed tick period, strategies process whatever comes | |
| Configurable | Default on engine, strategy can override per-tick | |

**User's choice:** Strategy decides

### Retraining callback handling

| Option | Description | Selected |
|--------|-------------|----------|
| Lifecycle hooks | Base class defines on_tick, on_retrain, on_complete hooks | ✓ |
| Strategy-internal | Strategies manage retraining inside tick() | |
| You decide | Claude picks cleanest approach | |

**User's choice:** Lifecycle hooks
**Notes:** User noted that strategies sometimes receive pre-trained data via CLI (for testing) — production uses lifecycle hooks. Must be compatible with trading_engine_optimizer.py so retraining can be skipped/reduced during optimization runs.

### Optimizer refactoring scope

| Option | Description | Selected |
|--------|-------------|----------|
| Refactor now | Update optimizer to use new engine in this phase | ✓ |
| Later phase | Focus on strategies/engine first, adapt optimizer separately | |

**User's choice:** Refactor now

### Optimizer integration approach

| Option | Description | Selected |
|--------|-------------|----------|
| Standard interface | Optimizer creates strategies via base class, same as demos | |
| Objective wrapper | Thin objective() function wrapping standard interface for skopt | |
| You decide | Claude picks what works cleanest with skopt | ✓ |

**User's choice:** You decide

---

## Claude's Discretion

- Signal type unification approach
- Optimizer integration pattern with skopt
- Import structure (relative vs absolute, __init__.py)
- Migration approach
- Deprecated file handling
- Base class method signatures and return types
- Test fixture strategy

## Deferred Ideas

None
