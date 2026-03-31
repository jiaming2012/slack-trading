# Phase 22: Remaining Strategy Migrations - Discussion Log

> **Audit trail only.**

**Date:** 2026-03-31
**Phase:** 22-remaining-strategy-migrations
**Areas discussed:** Migration approach, BaseStrategy changes

---

## Migration Approach

**User's choice:** You decide on all — apply Phase 21 pattern mechanically to remaining 5 strategies.

## BaseStrategy Changes

| Option | Description | Selected |
|--------|-------------|----------|
| Add on_signal() hook | Standardize signal consumption in base class | ✓ |
| Keep as-is | Each V2 handles independently | |
| You decide | | |

**User's choice:** Add signal hooks to BaseStrategy (Recommended)

## Claude's Discretion
- Migration order, datasource designs, on_signal() abstract vs default, demo updates, PDFWheel inheritance
