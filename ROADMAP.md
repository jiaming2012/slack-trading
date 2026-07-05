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

## Current state (2026-07-04, expanded from codebase scan)

- **Archived changes:** 0 (OpenSpec adopted today; 5 pre-OpenSpec refactor items shown green from git history)
- **In flight:** 16 — the overnight-run batch, drafted 2026-07-04 and validated strict; awaiting batch operator sign-off before implementation (see `todo/overnight-run-plan-20260704.md`)
- **Planned:** 14 cards (see "Card provenance" below for the 2026-07-04 expansion)
- **Next up:** `reconcile-models-packages` — merge `src/go/models` (37 files) and `src/go/eventmodels` (268 files) into one `models` package via per-type canonical selection with diff-testing. Detailed problem statement in `.planning/todos/pending/2026-07-04-reconcile-models-and-eventmodels-into-one-package.md`; sequencing rationale in `todo/next-steps-plan.md`.
- **Sequencing:** `reconcile-models-packages` → `rename-event-packages` (so imports rewrite once) → `migrate-crossed-enums` (the enums live in the packages being merged), with `migrate-live-account-type-data` riding alongside the enum migration (the legacy `mock`/`simulator` values are persisted in Postgres and need a data migration, not just a code change). Trading stack v4 follows the architecture doc's build order: schema → simulator adaptation → scanner L1–L2 → **safety rails gating any live trading** → fidelity checker → strategy optimizer → EV tracker → scanner optimizer → recommendation engine (see `todo/full-trading-stack-architecture.md` and `todo/trading-stack-gap-analysis.md`).
- **Just shipped (pre-OpenSpec):** the ADR-0001..0003 refactor series — broker seam, mode-blind clients, dead Tradier feed removal, `PlaygroundConfig` constructor, `DatabaseService` split into four stores.

## Card provenance (2026-07-04 expansion)

The map was expanded from 12 to 30 planned cards after a codebase scan. Every new card traces to a written source:

**From `HANDOFF.md` queued work (Architecture refactor area):**
- `migrate-live-account-type-data` — legacy `LiveAccountType` values persisted in the DB need a data migration (new "Mode presets" sibling of `migrate-crossed-enums`)
- `close-gorm-db-leaks` — `gorm.DB` leaks through `IDatabaseService`, annotated in `data/` code (new "Data layer hygiene" group)
- `e2e-sim-smoke-test` — the post-refactor end-to-end sim run (server + demo strategy) the handoff flagged as never verified (new "Refactor verification" group)

**From `todo/full-trading-stack-architecture.md` (Trading stack v4):**
- The old single `optimizers` card is split into `strategy-optimizer` and `scanner-optimizer` — they are separate build-order phases (4 and 6) with different data dependencies (sim_outcomes volume vs. EV weights)
- `scanner-ml-ranking` — the scanner's Layer 3 (XGBoost scoring) is a distinct capability from the L1–L2 filters/features already carded
- `temporal-scan-orchestration` — durable workflow per scan cycle (Temporal Go SDK); a new dependency, so its own change
- `optimizer-validation-pipeline` — the pre-training gate (timestamp audit, distribution check, regime confidence filter, fidelity gate, EV weight join)
- `recommendation-engine` — build-order phase 7, needs 6–12 months of EV history (new "Recommendations" group)

**From `todo/trading-stack-gap-analysis.md` (priority-ranked findings):**
- P1: `portfolio-risk-overlay` (finding 1), `net-ev-cost-model` (finding 7), `continuous-fidelity-monitoring` (finding 6)
- P2: `regime-robustness-layer` (finding 4), `overfitting-countermeasures` (finding 5), `shadow-config-deployment` (finding 8)
- P3: `feed-health-staleness` (finding 9), `esdb-role-decision` (finding 10), `crowding-detection` (finding 11), `db-partitioning-retention` (finding 12)
- The two P0 findings (kill switch / circuit breaker, broker-side stops) were already covered by the existing `kill-switch-and-broker-side-stops` card

Not carded (deliberately): operator-only actions with no code change — rotating the leaked Google service-account key, moving `vultr_ml_id_rsa` / `my-new-sealedsecrets.pem` out of the repo root, running a Paper session before Margin, and the CLAUDE.md layout-section refresh (docs chore, exception class).

## Pointers

- `HANDOFF.md` — session-resume notes (read by `/spec-next`)
- `CONTEXT.md` — domain glossary (canonical vocabulary)
- `docs/adr/` — architecture decision records 0001–0003
- `docs/architecture-review-20260704.html` — the 2026-07-04 architecture review that motivated the ADR series and the "Architecture refactor" cards (broker seam, DatabaseService split, mode-blind clients, dead Tradier feeds, Playground construction, DTO extraction)
- `openspec/` — spec-driven change workflow (see CLAUDE.md "Spec-first discipline")
- `.planning/` — archived GSD-era planning state (v1.0–v3.0 milestones; historical reference only)
