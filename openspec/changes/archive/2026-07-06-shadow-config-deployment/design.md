# Design — shadow-config-deployment

## Context

The architecture doc versions scanner configs in `scanner_configs` with rollback by id and says the scanner hot-swaps configs per cycle, but it never specifies how a *candidate* config earns trust before becoming the polled version. `scanner-optimizer` (sibling change) creates gate-validated proposals in `scanner_config_proposals` with an operator-only promote command; the operator's evidence at promote time is currently limited to derivation statistics and the overfitting verdict. This change inserts the missing evidentiary step: run the candidate in shadow — same inputs, parallel decisions, persisted divergence and outcomes — in Simulation only.

A key constraint discovered in the code: the `scanner` package's Layer 1 thresholds are compile-time constants (`hard_filters.go`, `regime_thresholds.go`) — the live scanner does not consume `ScannerConfig` payloads at all yet. Shadow execution therefore cannot mean "run the scanner twice with two configs". It means: replay the cycle's persisted scan observations (`scan_results` rows) through a payload-driven *decision engine*, once per config.

## Goals

- Same-inputs-by-construction: one observation set, two payloads, two decision vectors — no timing or data skew between the sides.
- Deterministic and pure at the core: the decision engine and divergence comparison are functions of `(payload, observations)`; all I/O lives in a thin loader/store layer.
- Every shadow run persisted: the evidence must survive for the operator's promotion decision and for later audit ("what did we know when we promoted?").
- Hard Simulation-only and read-only-except-own-tables guarantees, testable in code.

## Non-Goals

- Promotion or any apply path — that stays exclusively `scanner-optimizer`'s operator `promote` command; this change never mutates `scanner_configs`, proposals, or proposal statuses.
- Making the live scanner config-driven (hot-swap polling) — named future change `scanner-config-hot-swap`; this change's decision engine is the semantics that integration should reuse.
- Queueing shadow-only selections for simulation so they gain outcomes — requires a simulator work-queue integration that does not exist as a seam yet; deferred (named future change `shadow-outcome-backfill`). Outcome comparison is therefore coverage-explicit, not coverage-complete.
- Surfacing tickers the active pipeline never persisted (a shadow config that would loosen Layer 1) — impossible while observations replay persisted `scan_results`; explicitly reported as a limitation in the run report. Also addressed by `scanner-config-hot-swap` later.
- Paper/Margin operation of any kind; scheduling; new infra.
- New metrics/alerts. If added later they MUST use the internal telemetry registry (`src/go/telemetry` Counter/Gauge, AlertEngine) per ADR-0005 — never OpenTelemetry (removed). The persisted `shadow_runs` rows are the durable record; a one-shot CLI's in-memory counters would die with the process.
- ESDB events (ADR-0004 pending) — Postgres only.

## Decisions

### D1 — Shadow = replay through a payload-driven decision engine, not a second scanner run

`EvaluateConfig(payload scannercfg.Payload, obs []Observation) []Decision` where `Observation` is a per-ticker snapshot lifted from a persisted `ScanResult` (ticker, scanned_at, regime_tag, volume_ratio, rsi_14, atr_pct, scanner_score — nullable fields as pointers). Decision stages per ticker:

1. **Admission**: the payload's `hard_filter_overrides` for the ticker's regime (`volume_ratio >= volume_ratio_floor`, `atr_pct <= atr_pct_ceiling`; an absent override or a nil feature value does not reject — overrides tighten, they do not re-run Layer 1).
2. **Scoring**: features named in the regime's `feature_weights` (minus `drop_features`) are min-max normalized to [0,1] *across the observation batch per feature*, then combined as the weighted sum. Nil features contribute 0 and are noted. Batch normalization makes heterogeneous feature scales (RSI 0–100 vs volume_ratio ≈ 1) commensurable and is deterministic within a run. *Alternative:* raw weighted sums — rejected, scale-dominated; *alternative:* the Layer 3 ML score — does not exist yet.
3. **Selection**: admitted AND score ≥ the regime's `score_threshold` (a regime model without a threshold selects all admitted), then global top-N by score (`global.top_n_candidates`), ties broken by ticker ascending for determinism.
4. A ticker whose regime has no `regime_models` entry in the payload is marked `no_model`: admitted-by-default, unscored, never selected — visible in the report rather than silently dropped. *Alternative:* treat no-model as select-all — rejected, it would flood top-N with unscored tickers.

Both payloads run through this same engine over the same `obs` slice — parallel decisions with zero input skew by construction.

### D2 — Divergence definition

`CompareDecisions(active, shadow []Decision) DivergenceReport`: selected-set membership is the unit of divergence. Report carries `TotalObservations`, `SelectedActive`, `SelectedShadow`, `SelectedBoth`, `ShadowOnly`, `ActiveOnly` (ticker lists with both sides' scores), and `DivergencePct = |symmetric difference| / max(1, |union of selected sets|) × 100`. Score deltas on commonly-selected tickers are carried in the report detail but do not count as divergence — the operator cares about *different candidates*, not different score decimals. Deterministic ordering (tickers ascending) everywhere.

### D3 — Outcome comparison is coverage-explicit

For each side's selected tickers, join to `sim_outcomes` of that cycle window via the observation's `scan_result_id`. Per side, report: selections, selections-with-outcomes (coverage count and pct), decided count, win rate, mean `pnl_pct`. Shadow-only selections will usually have outcomes anyway *today* (they came from persisted scan results the simulator processed), but nothing guarantees it — hence explicit coverage, never imputation. Comparison math mirrors `ev-computation` conventions (pnl > 0 win, < 0 loss, breakeven excluded from decided).

### D4 — Persistence model

- `shadow_runs`: `id` UUID PK, `created_at`, `active_config_id` (UUID, nullable — null means the built-in default payload was the baseline), `proposal_id` (UUID FK → `scanner_config_proposals.id`), `window_start`/`window_end` (the `scanned_at` half-open window replayed), `total_observations`, `selected_active`, `selected_shadow`, `selected_both`, `divergence_pct`, `outcome_summary_json` (JSONB, both sides' outcome comparison incl. coverage), `synthetic` boolean (fixture runs are persisted only in tests, but the flag keeps any accidental persistence honest).
- `shadow_divergences`: `id` UUID PK, `shadow_run_id` (UUID FK → `shadow_runs.id`), `ticker`, `kind` (text CHECK ∈ {`shadow_only`, `active_only`}), `active_score`/`shadow_score` (nullable numerics), `scan_result_id` (UUID, loose reference for drill-down).
- `MigrateShadowDeployment(db)` — additive, idempotent, creates only these two tables (the `migrate.go` DO-block pattern); requires `MigrateScannerOptimizer` first (FK target), erroring clearly otherwise. `ShadowStore` interface + GORM impl + in-memory fake (the `CrowdingStore` pattern).

### D5 — Simulation-only guard, enforced not assumed

The run entry point takes the operating mode explicitly and returns `ErrNotSimulation` unless it is Simulation (vocabulary per CONTEXT.md: Simulation | Paper | Margin). The CLI resolves mode from the environment the same way the server does and passes it in; tests cover the refusal for Paper and Margin. Depth of defense is cheap here because the blast radius is small anyway: the package writes only its own two tables and never touches broker/order paths — the guard exists so shadow evidence can never be mistaken for, or generated from, real-money context.

### D6 — Shadow candidates must be gate-validated

`RunShadow` refuses a proposal whose status is `rejected_by_gate` (error, no run row). `pending_review` is the expected input; `promoted` proposals are also allowed — re-running shadow on a promoted config against a fresh window is legitimate post-promotion monitoring. *Alternative:* pending-only — rejected as it would forbid that monitoring use.

### D7 — Active baseline resolution

Active payload = `scanner_configs` row with the latest `created_at` (parse its `config_json`); when the table is empty, `scannercfg.DefaultPayload()` with `active_config_id` NULL on the run row. This mirrors `scanner-optimizer`'s baseline rule so both changes agree on what "active" means before hot-swap exists.

## Risks → mitigations

- **Evidence looks authoritative but observations are active-pipeline-biased** (loosening configs can't surface unseen tickers) → limitation printed in every run report and recorded here; resolved structurally by `scanner-config-hot-swap` + `shadow-outcome-backfill` later.
- **Batch min-max normalization makes scores window-dependent** (same ticker, different window → different score) → scores are only ever compared *within* a run between the two sides, never across runs; the report never presents cross-run score comparisons.
- **Empty windows / no observations** → `RunShadow` returns a distinct sentinel (`ErrNoObservations`) and persists nothing, rather than a misleading zero-divergence run.
- **Nil-heavy observations** (old rows missing features) → nil handling pinned per stage (admission does not reject on nil; scoring contributes 0 and notes it); fixture tests cover nil-feature rows.
- **FK/migration ordering** → same convention as the sibling change: clear wrapped error naming the missing dependency table; test covers fresh-DB ordering.

## Migration / rollback

- Forward: run `MigrateOverfittingCountermeasures` → `MigrateScannerOptimizer` → `MigrateShadowDeployment` (each additive, idempotent, explicitly invoked — none wired into `dbutils.InitPostgres`).
- Rollback: drop `shadow_divergences` then `shadow_runs`; nothing else references them. No existing table, model, or the `scanner` package is altered by this change, so rollback has no behavioral blast radius.
