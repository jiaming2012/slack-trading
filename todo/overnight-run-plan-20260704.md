# Overnight Autonomous Run Plan — 2026-07-04

**Orchestrator:** Fable (main session, live overnight)
**Workers:** Sonnet / Opus subagents for marginal + well-specified work; Fable for risky decisions and all final review
**Human involvement:** ONE batched OpenSpec sign-off before the run starts. Zero during the run. Morning report at the end.
**Goal:** Cover as much of `usm/roadmap.txt` as possible with archive-quality OpenSpec changes, each gated by deterministic verification.

---

## Feasibility summary

| Bucket | Cards | Verdict |
|---|---|---|
| Fully completable overnight | 10 | Code + tests + gates + archive |
| Buildable, partial validation | 6 | Code + unit tests land; final validation deferred (external deps) |
| Blocked — not attempted | 8 | Live data, prod DB, external infra, or operator-only |
| Already done (green) | 5 | — |
| Decision-only | 1 | ADR draft produced, operator decides later |

Ceiling: ~16 of 30 planned cards touched; ~10–14 archived. The rest are physically impossible overnight, not a planning failure.

---

## Card triage

### Tier A — full completion expected (10 cards)

| Card | Model | Why eligible | Verification gate |
|---|---|---|---|
| `reconcile-models-packages` | **Fable** (not delegable — 47 canonical-selection judgment calls) | Well-specified in pending todo; MIG-03 diff-test pattern proven in Phase 21 | G1+G2+G3+G4 |
| `rename-event-packages` | Sonnet (mechanical) | Pure import/package rewrite | G1+G2+G4 |
| `close-gorm-db-leaks` | Opus | Leak sites already annotated in `data/` | G1+G2 + Fable diff review |
| `e2e-sim-smoke-test` | Opus | Harness + Taskfile target; AAPL 2025 dates work with Polygon | G5 (the test IS the gate) |
| `trading-stack-schema` | Sonnet | Greenfield GORM models + migrations, spec'd verbatim in `todo/full-trading-stack-architecture.md` | testcontainers round-trip tests + Fable review |
| `db-partitioning-retention` | Sonnet | Partitioning DDL for the NEW v4 tables only (never prod) | testcontainers + Fable review |
| `net-ev-cost-model` | Opus | Gap analysis sizes it at 2–3 days human; pure calc + schema | Unit tests w/ hand-computed fixtures |
| `ev-tracker` | Opus | Go + Postgres rolling aggregations; deterministic | Unit + testcontainers, synthetic trade fixtures |
| `fidelity-checker` | Opus | Deterministic comparison logic | Unit tests: synthetic sim-vs-live pairs with known drift |
| `crowding-detection` | Sonnet | ~2 days human; SQL over scan queue | Unit tests w/ fixture overlap scenarios |

### Tier B — build + unit-test overnight, final validation deferred (6 cards)

| Card | Model | What lands | What's deferred |
|---|---|---|---|
| `migrate-crossed-enums` | **Fable** | Code-side mode presets (Simulation/Paper/Margin) replacing `PlaygroundEnvironment`×`LiveAccountType` (38 files) | Prod data migration (see Tier C twin card); Paper session validation |
| `scanner-l1-l2` | Opus | Hard filters + feature extraction against fixture candle data | Live-feed run against real Polygon universe |
| `optimizer-validation-pipeline` | Opus | Timestamp audit, distribution check, regime filter, fidelity gate, EV join — all deterministic | Real training-cycle integration |
| `feed-health-staleness` | Sonnet | Heartbeat + staleness detection with fake clocks | Live feed outage drill |
| `portfolio-risk-overlay` | Opus | Pre-trade gate (exposure/concentration/drawdown checks) wired into sim path only | Live-mode enablement is operator-gated |
| `kill-switch-and-broker-side-stops` | Opus + **Fable review (safety-critical)** | Kill-switch API endpoint + anomaly guard w/ unit tests | Broker-side stop verification needs Tradier sandbox — NOT overnight |

### Tier C — not attempted overnight (8 cards + 1 decision)

- `migrate-live-account-type-data` — mutates production Postgres on the DO droplet. Never autonomous.
- `verify-grafana-heartbeat-and-stale-alert`, `close-v3-milestone` — Windows-desktop Grafana + operator milestone ritual.
- `temporal-scan-orchestration` — adopting new infra (Temporal) unsupervised is a bad idea; also needs a running Temporal server.
- `recommendation-engine` — needs 6–12 months of EV history that does not exist.
- `scanner-ml-ranking`, `strategy-optimizer`, `scanner-optimizer` — need labeled `scan_results ⋈ sim_outcomes` volume (≥500 rows) that only accrues after the scanner/simulator run for real. Building them against synthetic data overnight would produce untestable model code — skipped rather than faked.
- `regime-robustness-layer`, `overfitting-countermeasures`, `shadow-config-deployment` — layer on the optimizers above; same blocker.
- `esdb-role-decision` — decision, not code. Overnight output: a drafted ADR recommending Pattern B (Postgres primary, ESDB audit trail) with evidence from the codebase. Operator decides in the morning.

---

## Verification gates (the no-human substitute)

Every change must pass its gates before commit; every commit carries the `OpenSpec-Change:` trailer.

- **G1** `go build ./src/go/... ./cmd/...` green
- **G2** `task test` green
- **G3** Python: `pytest` failure set is a subset of the Wave-0 baseline (179 known pre-existing failures — gate fails only on NEW failures)
- **G4** MIG-03 diff-test: fixed backtest scenario, byte-for-byte output identical to the Wave-0 reference capture
- **G5** E2E smoke: server boots, demo strategy completes a full tick loop, orders fill
- **G6** Fable adversarial review: for every Sonnet/Opus diff, a review pass by the orchestrator (or a max-effort review agent) that must explicitly approve before commit. Safety-rails code (kill switch, risk overlay) gets G6 unconditionally.

**Repair policy:** a failed gate gets ONE repair attempt by the authoring agent, then ONE by Fable. Still failing → revert that change's commits entirely (atomic per-change commits make this clean), mark the card failed in the morning report, and skip all dependents. Never leave the tree partially migrated.

**Hard prohibitions overnight:** no `git push`, no deploys, no prod DB connections, no Tradier live/paper orders, no infra provisioning, no force operations. Local Docker (Postgres/ESDB testcontainers, `task db:start`) is allowed.

---

## Execution schedule

### Phase 0 — evening, WITH operator (~30–45 min, the one human touch)

1. Fable drafts OpenSpec changes (proposal + spec deltas + tasks; design.md where cross-cutting) for all Tier A + Tier B cards; `openspec validate` each.
2. Roadmap cards flip to yellow `#FFF59D` in the same drafts.
3. **One consolidated plain-English sign-off** covering the whole batch, per CLAUDE.md step 3. Operator replies once (yes / drop card X / discuss).
4. Operator starts the session in a no-prompt permission mode and confirms: `.env` present (Polygon key), Docker up, machine set to not sleep.

### Phase 1 — baseline capture (Wave 0, ~30 min, Fable)

- Record: `go build` + `task test` green at HEAD; pytest baseline failure list (exact test IDs); MIG-03 reference output from a fixed backtest scenario; create `claude/overnight-20260704` branch.
- Any baseline check failing → run scope shrinks to whatever doesn't depend on the broken gate; noted in morning report.

### Phase 2 — refactor track (serialized — everything downstream depends on it)

1. `reconcile-models-packages` (Fable): inventory → canonical-selection table → merge → import rewrite (32+ importers) → G1–G4. Biggest single block of the night (est. 3–5 h wall clock). **If this fails, the refactor track halts and the night pivots to Phase 3 only** (v4 greenfield doesn't strictly need it).
2. `rename-event-packages` (Sonnet, worktree) → G1+G2+G4 + G6.
3. `close-gorm-db-leaks` (Opus) → G1+G2 + G6.
4. `migrate-crossed-enums` (Fable) → G1–G4. Code-side only.
5. `e2e-sim-smoke-test` (Opus) → G5 becomes a permanent Taskfile target.

### Phase 3 — trading stack v4 greenfield (parallel fan-out, worktree isolation)

New packages, low collision surface; rebased onto the post-refactor tree (or onto HEAD if Phase 2 halted).

- **Wave 3a (parallel):** `trading-stack-schema` → then `db-partitioning-retention` chained on it. Alongside: `fidelity-checker`, `ev-tracker`, `feed-health-staleness` (interfaces stubbed against the schema change's models).
- **Wave 3b (parallel, after 3a merges):** `net-ev-cost-model`, `crowding-detection`, `optimizer-validation-pipeline`, `scanner-l1-l2`, `portfolio-risk-overlay`, `kill-switch-and-broker-side-stops`.
- Each: author (Sonnet/Opus per triage) → gates → G6 Fable review → merge to run branch → archive change → flip roadmap card green. Integration re-run of G1–G3 after every merge.
- `esdb-role-decision` ADR drafted by Opus during idle capacity.

### Phase 4 — closeout (Fable, ~1 h)

- Full gate sweep (G1–G5) on the final tree.
- Archive every completed change (`openspec archive`); flip roadmap cards; refresh ROADMAP.md current-state; update HANDOFF.md with a full morning report: per-card outcome (archived / landed-partial / failed-reverted / skipped), gate evidence, and the exact revert commits for anything rolled back.
- Branch left local and unpushed for morning inspection.

---

## Model economics

- **Fable:** orchestration, reconcile + enum migration, all G6 reviews, all merge/halt decisions. High token cost, applied only where judgment is irreplaceable.
- **Opus:** substantial implementation against tight specs (fidelity checker, EV tracker, risk overlay, kill switch).
- **Sonnet:** mechanical work with fully deterministic gates (renames, schema/DDL, fixtures, SQL, staleness timers).
- Nothing authored by Sonnet/Opus reaches a commit without a deterministic gate AND (for anything non-mechanical or safety-adjacent) a G6 Fable review. Marginal work is cheap precisely because its validation is not.

## Risk register

| Risk | Mitigation |
|---|---|
| Reconcile merge goes sideways mid-night | Atomic commits + full revert policy; v4 track proceeds independently |
| Context window churn over ~8 h | Harness auto-summarizes; plan + per-card state live in files (openspec tasks.md), not in conversation memory |
| Permission prompt silently blocks at 2 AM | Operator starts session in no-prompt mode; Phase 0 checklist verifies with a canary command |
| Polygon API failures | Only `e2e-sim-smoke-test` and `scanner-l1-l2` fixtures need it; both degrade to skipped-with-note |
| Sonnet produces plausible-but-wrong code | Never committed on green-build alone: tests written against the spec (not the implementation) + G6 review |
| Token burn-out mid-run | Optional budget directive at kickoff; Phase 2 prioritized over Phase 3 so the highest-value work happens first |
