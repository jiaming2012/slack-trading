# Next Steps Plan

**Prepared:** 2026-07-04 (revised after pulling origin/dev @ `96507039`)
**Handoff inputs:** dev-branch refactor series (ADRs 0001–0003, `CONTEXT.md` glossary), pending todo `.planning/todos/pending/2026-07-04-reconcile-models-and-eventmodels-into-one-package.md`, `todo/` trading-stack docs, `.planning/` GSD state

---

## Where Things Stand

The dev branch carries a target-architecture refactor series, now pulled locally:

- **Done on dev:** Broker seam routing every order/tick (ADR-0001), blocking `tick()` with client-side mode branches deleted (ADR-0002 — `is_live` is gone from the Python engine), dead Tradier feed paths removed (ADR-0003), `PlaygroundConfig` replacing the 13-param constructor, `DatabaseService` split into four stores, proto converters extracted, deprecated tree removed, domain glossary (`CONTEXT.md`) + 3 ADRs in `docs/adr/`.
- **Explicit handoff (pending todo, captured 2026-07-04):** reconcile `src/go/models` (37 files) and `src/go/eventmodels` (268 files) into ONE `models` package — 47 colliding type names, 14 same-name files with diverged content, 32 importers of `src/go/models`. Sized as its own phase; too risky as a quick task.
- **Remaining ADR-0001 debt (verified in code):** the crossed enums `PlaygroundEnvironment` × `LiveAccountType` are still live — `LiveAccountType` appears in 38 files. ADR-0001 declares this legacy to migrate to mode presets.

---

## Step 1 — Models/Eventmodels Reconciliation Phase (the handoff item)

Add it as a phase (`/gsd-phase add`), then `/gsd-discuss-phase` → `/gsd-plan-phase` → `/gsd-execute-phase`. Per the todo's own plan:

1. **Classify the 47 colliding types**: which copy is live on current code paths (backtester, live trading) vs legacy-only (old Slack framework: price levels, strategies v1).
2. **Pick canonical copy per type**, port unique methods from the loser, land everything in one package named `models` (event-driven naming dies).
3. **Rewrite imports** — 32 importers of `src/go/models`, all importers of `eventmodels`. Note `eventmodels` itself imports `models` in places (`bottraderequest.go`).
4. **Diff-test before/after** using the MIG-03 pattern from Phase 21: fixed backtest scenario, byte-for-byte output comparison.
5. Establish a baseline first: `go build ./...`, `task test` green before starting; run the diff scenario on HEAD to capture reference output.

**Plan-shape suggestion:** split into ~3 plans — (a) inventory + canonical-selection table with diff-test harness, (b) the merge + import rewrite, (c) verification + cleanup. Steps here bulk-edit 300+ files; worktree isolation recommended.

## Step 2 — Package Renames (same spirit, separate plan or phase)

From the todo's follow-up list: `eventservices`→`marketdata`, `eventconsumers`→`workers`, `eventproducers`→`api`, `eventpubsub`→`pubsub`, `backtester-api`→`backtester`. Then update the CLAUDE.md layout section (and `.planning/codebase/` docs). Mechanical but wide — do it *after* Step 1 lands so imports only get rewritten once per file where possible.

## Step 3 — Crossed-Enum Migration (finish ADR-0001)

Replace `PlaygroundEnvironment` × `LiveAccountType` with the three mode presets (Simulation, Paper, Margin) per the glossary. 38 files reference `LiveAccountType` today. This also retires the "reconcile playground" operator-facing concept per `CONTEXT.md`. Candidate for the phase after Steps 1–2, since the enum types themselves live in the packages being merged/renamed — sequencing them first avoids double churn.

## Housekeeping (fits anywhere)

- `STATE.md` still says "Executing Phase 26" though Phase 26 verified passed and audit gaps were fixed (quick task `260401-fij`) — run `/gsd-complete-milestone` to close v3.0 properly.
- Two Phase 26 human verifications never ran (heartbeat gauge E2E; stale alert firing) — verify against the Windows-desktop Grafana (`http://100.70.200.55:3000`).

## Later — v4.0 Trading Stack

`todo/full-trading-stack-architecture.md` + `todo/trading-stack-gap-analysis.md` define the five-component feedback-loop system (scanner, simulator, optimizers, fidelity checker, EV tracker). That becomes its own milestone (`/gsd-new-milestone`) once the refactor track lands — the models consolidation directly de-risks it, since the new components build on the same Go domain types. Build order: schema → simulator adaptation → scanner L1–2 → **safety rails (P0: kill switch, broker-side stops) gating any live trading** → fidelity checker → EV tracker → optimizers.

---

## TL;DR

1. Pull was done — local dev is now at `96507039`.
2. Next work item is the **models/eventmodels reconciliation phase** (the pending todo): per-type canonical selection, one `models` package, import rewrite, MIG-03-style diff testing.
3. Then package renames, then the crossed-enum → mode-preset migration to finish ADR-0001.
4. v3.0 closeout housekeeping is pending; the v4.0 trading stack milestone comes after the refactor track.
