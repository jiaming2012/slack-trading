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

## Current state (2026-07-05, post overnight autonomous run)

- **Archived changes:** 17 — the overnight batch plus `repair-python-test-suite` (2026-07-05: 179-failure baseline retired, suite green 416/0, `task test:python` + `task test:demo-covered-call` added)
  - the entire overnight batch shipped, gate-verified, adversarially reviewed, and archived under `openspec/changes/archive/2026-07-05-*` (see `HANDOFF.md` for the full morning report and per-change ledger)
- **In flight:** 0
- **Planned:** 18 cards — the 14 pre-run cards plus 4 follow-ups born from the overnight adversarial reviews: `widen-scan-results-columns` (3 scanner features computed but unpersisted — schema gap inherited from the architecture doc), `wire-anomaly-guard-feeds` (guards are tested components, live observation feeds unwired), `wire-companion-stops` (broker-held stop library tested vs MockBroker, fill-pipeline invocation deferred), `wire-risk-overlay-state` (risk-gate engine complete, portfolio-state mapping + Simulation enablement deferred)
- **Operator decisions pending:** ADR-0004 EventStoreDB role (drafted, recommends Pattern B — Postgres primary), pushing `claude/overnight-20260704` (108 local commits), `migrate-live-account-type-data` (never autonomous)
- **Next up:** the `wire-*` trio connecting the tested safety components to live data — `wire-anomaly-guard-feeds`, `wire-companion-stops`, `wire-risk-overlay-state` — which gate any future Paper/Margin trading.
- **Sequencing:** the refactor track and v4 foundations are done; remaining v4 order per the architecture doc: simulator adaptation → scanner ML ranking (needs labeled data volume) → strategy optimizer → scanner optimizer → recommendation engine, with the regime/overfitting/shadow-deploy hardening trio alongside the optimizers.
- **Just shipped (2026-07-05):** the 16-change overnight batch — models-package reconciliation, package renames, gorm de-leak, Mode/AccountRole migration, e2e smoke gate, the six-table v4 schema + partitioning, fidelity checker, EV tracker + net-EV cost model, scanner L1–L2, optimizer validation pipeline, feed health, crowding detection, kill switch, and the portfolio risk overlay.

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
