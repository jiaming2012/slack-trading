# Roadmap

The roadmap source of truth is **`usm/roadmap.txt`** — a TextUSM user story map. Activity rows are capability areas, user-task rows are sub-capabilities, and each story card names exactly one OpenSpec change. This file is the narrative companion: it explains the map and never contradicts it.

## Viewing

Copy `usm/roadmap.txt` to the clipboard (`cat usm/roadmap.txt | clip.exe` on WSL), paste into [app.textusm.com](https://app.textusm.com), and pick **User Story Map** in the diagram-type dropdown. Do not trust the VS Code TextUSM extension — verify rendering in the web client only.

## Color legend (status)

| Color | Hex | Meaning |
|---|---|---|
| 🟢 Light green | `#C5E1A5` | Done — OpenSpec change archived under `openspec/changes/archive/` |
| 🟡 Light yellow | `#FFF59D` | In flight — folder exists under `openspec/changes/`, not yet archived |
| ⬜ White | `#FFFFFF` | Planned — named but not yet drafted (no folder) |
| 🔴 Light red | `#EF9A9A` | Blocked — external dependency prevents progress (rare) |
| ⚪ Light gray | `#E0E0E0` | Deferred — was on the plan, removed (rare; explained here when used) |

**Pre-OpenSpec exception:** the green cards under "Target architecture ADRs" shipped before this project adopted OpenSpec (2026-07-04) — they were done under the previous GSD workflow and are verified in git history (`docs/adr/0001..0003`, commits `fdc45f4..9ca094e`), not in `openspec/changes/archive/`. Every card after OpenSpec adoption maps 1:1 to an OpenSpec change slug.

## Freshness discipline

Every OpenSpec change updates `usm/roadmap.txt` **in the same change set**:

- **Draft** a change → add or flip its card(s) to `#FFF59D` (in flight)
- **Archive** a change → flip its card(s) from `#FFF59D` to `#C5E1A5` (done)
- **Scope grows** → add cards under any newly touched user-task group

When flipping cards, also refresh the "Current state" section below and date-stamp it.

## Current state (2026-07-04)

- **Archived changes:** 0 (OpenSpec adopted today; 5 pre-OpenSpec refactor items shown green from git history)
- **In flight:** 0
- **Planned:** 12 cards
- **Next up:** `reconcile-models-packages` — merge `src/go/models` (37 files) and `src/go/eventmodels` (268 files) into one `models` package via per-type canonical selection with diff-testing. Detailed problem statement in `.planning/todos/pending/2026-07-04-reconcile-models-and-eventmodels-into-one-package.md`; sequencing rationale in `todo/next-steps-plan.md`.
- **Sequencing:** `reconcile-models-packages` → `rename-event-packages` (so imports rewrite once) → `migrate-crossed-enums` (the enums live in the packages being merged). Trading stack v4 follows the build order schema → simulator adaptation → scanner → **safety rails gating any live trading** → fidelity checker → EV tracker → optimizers (see `todo/full-trading-stack-architecture.md` and `todo/trading-stack-gap-analysis.md`).
- **Just shipped (pre-OpenSpec):** the ADR-0001..0003 refactor series — broker seam, mode-blind clients, dead Tradier feed removal, `PlaygroundConfig` constructor, `DatabaseService` split into four stores.

## Pointers

- `HANDOFF.md` — session-resume notes (read by `/spec-next`)
- `CONTEXT.md` — domain glossary (canonical vocabulary)
- `docs/adr/` — architecture decision records 0001–0003
- `docs/architecture-review-20260704.html` — the 2026-07-04 architecture review that motivated the ADR series and the "Architecture refactor" cards (broker seam, DatabaseService split, mode-blind clients, dead Tradier feeds, Playground construction, DTO extraction)
- `openspec/` — spec-driven change workflow (see CLAUDE.md "Spec-first discipline")
- `.planning/` — archived GSD-era planning state (v1.0–v3.0 milestones; historical reference only)
