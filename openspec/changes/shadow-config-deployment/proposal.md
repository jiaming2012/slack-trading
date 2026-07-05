# Shadow Config Deployment (Simulation-only evidence before promotion)

## Why

A gate-passing scanner-config proposal is still just a promise — the operator has only derivation-time statistics to judge it by; this change lets a validated proposal run in shadow alongside the active config over the exact same Simulation inputs, with every decision divergence and available outcome comparison persisted, so a proposal earns observed evidence before the operator decides to promote it.

## What Changes

- Add a Go package `src/go/tradingstack/shadowdeploy/` that evaluates **two** scanner-config payloads — the active one (latest `scanner_configs` row, or the built-in default payload when none exists) and a shadow candidate (a `scanner_config_proposals` row) — over the **same** set of scan observations (persisted `scan_results` rows in a chosen `scanned_at` window), producing parallel per-ticker decisions: admitted (per the payload's regime `hard_filter_overrides`), scored (payload `feature_weights` over batch-normalized features), and selected (score threshold + top-N).
- Compute a deterministic **divergence report**: tickers selected by the shadow config only, by the active config only, by both, and a divergence percentage — plus an **outcome comparison** joining each side's selections to whatever `sim_outcomes` already exist (win rate, mean pnl of covered selections, with coverage reported explicitly; outcomes are never fabricated for uncovered tickers).
- Persist evidence to two new Postgres tables via a dedicated additive migration (`MigrateShadowDeployment`): `shadow_runs` (configs compared, window, totals, divergence pct, per-side outcome summary) and `shadow_divergences` (one row per diverging ticker with both sides' scores).
- **Simulation-only, read-only guard**: the shadow runner refuses to execute unless the operating mode is Simulation; it reads `scan_results`/`sim_outcomes`/`scanner_configs`/`scanner_config_proposals` and writes only its own two tables — it places no orders of any kind and never mutates the active config or any proposal's status. It also refuses shadow candidates whose proposal status is `rejected_by_gate` (only gate-validated proposals earn shadow evidence).
- Add operator commands (`cmd/shadow-scan/main.go`: `run` with a `--synthetic` fixture self-check mode, and `report` to print a persisted run's divergence and outcome evidence) and Taskfile targets `task scanner:shadow`, `task scanner:shadow-report`, `task test:shadow-deployment`.
- Promotion remains exactly where `scanner-optimizer` put it: the operator consults the shadow report, then (or not) runs `task scanner:promote`. This change adds **no** promotion or apply path.
- No new metrics or alerts (any future ones must use the internal telemetry registry per ADR-0005 — never OTel); Postgres persistence only (ESDB untouched, ADR-0004 pending). **Not BREAKING.**

## Capabilities

### New Capabilities

- `shadow-config-deployment`: the two-config parallel decision engine over identical Simulation inputs, the persisted divergence/outcome evidence, the Simulation-only guard, and the operator run/report commands.

### Modified Capabilities

(none)

## Impact

- New package: `src/go/tradingstack/shadowdeploy/` (decision engine, divergence + outcome comparison, GORM models + migration, store interface + fake, synthetic fixtures, tests). New command `cmd/shadow-scan/`. Only `taskfile.yml` is edited among existing files.
- **Sequencing**: depends on `scanner-optimizer` (imports the `scannercfg.Payload` type and reads `scanner_config_proposals`; migration FKs the proposal table) which in turn depends on `overfitting-countermeasures`. This is the last of the three to implement. Data dependencies on `trading-stack-schema` tables (`scan_results`, `sim_outcomes`, `scanner_configs`).
- The `scanner` package is untouched: shadow evaluation replays persisted scan observations through the payload-driven decision engine rather than re-running Layer 1/2 (whose thresholds are compile-time constants today). When the future `scanner-config-hot-swap` change makes the live scanner config-driven, the same engine semantics apply.
- Autonomous-run safe: deterministic unit tests plus testcontainers round-trips; `run --synthetic` needs no database; Simulation-only guard enforced in code and tested; no live/paper orders, no prod DB, no push, no new infra.
- **Conservative scope decisions needing sign-off** (detail in design.md): shadow decisions replay persisted `scan_results` (so only tickers the active pipeline persisted are observable — a shadow config that would *loosen* Layer 1 admission cannot surface never-scanned tickers yet); scoring uses batch min-max normalized features because no Layer 3 ML score exists; outcome comparison covers only selections that already have `sim_outcomes` rows (shadow-only selections are not queued for simulation in this change).
