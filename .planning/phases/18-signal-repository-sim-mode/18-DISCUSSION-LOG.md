# Phase 18: Signal Repository & Sim Mode - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.

**Date:** 2026-03-31
**Phase:** 18-signal-repository-sim-mode
**Areas discussed:** Repository interface, Signal delivery, Clock gating, ESDB persistence

---

## Repository Interface Design

| Option | Description | Selected |
|--------|-------------|----------|
| Per-symbol with name filter | Write/Read scoped per symbol | |
| Per-playground | Playground owns signals | |
| Hybrid | Per-symbol storage, per-playground delivery | |

**User's choice:** Custom — "Per-symbol with name filtering. However, symbols should not be in separate streams. Instead, the symbol should be an attribute to the TradeSignal event (on the same level as the name)."
**Notes:** Single global signal stream, NOT per-symbol streams. This overrides the research recommendation. Symbol is a filter attribute, not a partition key.

---

## Signal Delivery via TickDelta

| Option | Description | Selected |
|--------|-------------|----------|
| Add NewSignals to TickDelta | Same pattern as candles | ✓ |
| Separate GetSignals RPC | Extra round-trip per tick | |
| You decide | | |

**User's choice:** Add NewSignals to TickDelta (Recommended)

---

## Clock Gating

| Option | Description | Selected |
|--------|-------------|----------|
| FIFOQueue with clock cutoff | Proven candle pattern | |
| Repository cursor | More flexible, adds state | |
| You decide | | ✓ |

**User's choice:** You decide (FIFOQueue recommended)

---

## ESDB Persistence Flag

| Option | Description | Selected |
|--------|-------------|----------|
| Write-through on each signal | Immediate for live | |
| Batch at sim end | On SavePlayground | |
| You decide | | |

**User's choice:** Custom — "Write-through on each signal for live playgrounds. Batch at sim end if the playground is a simulator and the SavePlayground method is invoked."
**Notes:** Dual behavior: live = automatic write-through, sim = batch only on SavePlayground (existing --save-to-db flag)

---

## Claude's Discretion

- FIFOQueue implementation for signal delivery
- ISignalRepository method signatures
- Stream naming for persisted signals
- ReadPending return type

## Deferred Ideas

- ESDBSignalRepository (Phase 20)
- WriteSignal RPC (Phase 19)
- Signal replay (Phase 23)
