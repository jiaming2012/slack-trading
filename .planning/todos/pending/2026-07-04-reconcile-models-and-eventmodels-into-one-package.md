---
created: 2026-07-04T14:38:59.929Z
title: Reconcile models and eventmodels into one package
area: general
files:
  - src/go/models/
  - src/go/eventmodels/
---

## Problem

`src/go/models` (37 files) and `src/go/eventmodels` (268 files) both exist because the system was originally headed toward an event-driven architecture that was later abandoned. They are not mergeable mechanically: 47 type names collide (`Trade`, `Account`, `Candle`, `Strategy`, `PriceLevel`, `SignalV2`, ...), and the copies have **diverged** — 14 files with the same name have different content (method sets differ), 19 colliding types live in differently-named files, and only 4 files are identical. `eventmodels` itself imports `models` in places (e.g. `bottraderequest.go`). 32 files across the repo import `src/go/models`. Three packages currently answer to the name "models" (`src/go/models`, `src/go/eventmodels`, `src/go/backtester-api/models`), which misleads both new devs and AI tooling.

## Solution

Per-type reconciliation, not a bulk merge:

1. For each of the 47 colliding types, determine which copy is live on current code paths (backtester, live trading) vs legacy-only (old Slack-trading framework: price levels, strategies v1).
2. Pick the canonical copy, port any unique methods from the loser, land everything in ONE package named `models` (event-driven naming dies per the abandoned direction).
3. Rewrite imports (32 importers of `src/go/models`, all importers of `eventmodels`).
4. Diff-test before/after (MIG-03 pattern from Phase 21 proved this works) — run a fixed backtest scenario and compare outputs byte-for-byte.
5. Follow-up in the same spirit: rename `eventservices`→`marketdata`, `eventconsumers`→`workers`, `eventproducers`→`api`, `eventpubsub`→`pubsub`, `backtester-api`→`backtester`; update CLAUDE.md layout section.

Sized as its own phase; too risky as a quick task.
