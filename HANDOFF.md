# Session Handoff — Architecture Deepening + Repo Cleanup

**Date:** 2026-07-04 · **Branch:** `dev` (local, **11 commits ahead of `origin/dev` — NOT pushed**)

## What this session did

Read these first: `CONTEXT.md` (domain glossary — canonical vocabulary: Playground, Mode = Simulation|Paper|Margin, Broker seam, Reconciliation, Feed, Tick) and `docs/adr/0001..0003` (target architecture decisions). All work below implements or serves those ADRs. GSD was explicitly bypassed for this work with operator permission.

### Commits on dev (oldest first)

| Commit | What |
|--------|------|
| `a4d0d94` | Domain glossary (CONTEXT.md) + ADRs 0001-0003 |
| `fdc45f4` | Deleted dead Tradier feed paths (ADR-0003). NOTE: `RepositorySourceTradier` is NOT dead — persisted live-repo marker, documented in `data/database_service.go` CreateRepos |
| `28fbbf8` | `NewPlayground` now takes a `PlaygroundConfig` struct (was 13 positional params); 59 call sites rewritten |
| `17eb1ee` | **Broker seam (ADR-0001):** `models/broker_seam.go` defines `Broker` interface; adapters `SimulatedBroker`/`LiveBroker`/`ReconcileBroker` in own files; `Playground.PlaceOrder`/`Tick` env switches collapsed to `brokerFor()` dispatch; bodies moved verbatim |
| `c2c1177` | `router/proto_converters.go` owns all model→proto mapping (18 funcs); handlers thin (NextTick 181→72 lines) |
| `dbb9195` | **Mode-blind clients (ADR-0002):** `tick()` blocks in every mode; new `_wait_until_next_tick` + `on_wait` callback in `engine/client.py`; removed poll loop in `mean_reversion_v2.py`, `is_live` span gating, `PLAYGROUND_ENV`-derived log levels |
| `9ca094e` | `DatabaseService` split into `playground_store.go`/`order_store.go`/`live_account_store.go`/`equity_store.go`; public surface unchanged (51 methods); locking granularity unchanged |
| `e748d49` | Deleted `deprecated/` (130 files), old-tree strays, `eventmain` binary, tracked `js/node_modules`; **untracked `credentials.json`** |
| `763d012` | Loguru sinks write to `logs/` not repo root |
| `9650703` | GSD todo: models/eventmodels reconciliation (see below) |

### Deliberate behavior changes (not bugs)
1. Log verbosity from `LOG_LEVEL` env (default INFO) — live no longer auto-TRACE. Set `LOG_LEVEL=TRACE` on the DO droplet to restore.
2. Live idle ticking now matches sim policy: HTF advance when no active groups (was always-LTF). Intentional ADR-0002 alignment.
3. OTel spans emit in Simulation too (were live-only).

## Verified / NOT verified
- ✅ `go build ./src/go/... ./cmd/...` + `task test` green at every commit
- ✅ Python: 245 passed / 179 failed — **the 179 pre-exist identically at the pre-refactor HEAD** (verified via stash); many are in mock-env tests
- ❌ Paper/Margin live paths (Tradier) — compile-clean only. **Run a Paper session before Margin.**
- ❌ End-to-end sim smoke test (server + demo strategy) — recommended next step

## Urgent / security
- **`credentials.json` was git-tracked until `e748d49` — it is in git history. ROTATE the Google service-account key.** File still on disk, now ignored.
- `vultr_ml_id_rsa` (SSH private key) and `my-new-sealedsecrets.pem` sit untracked in repo root — move out.

## Open items (operator decisions pending)
- **Push `dev`** — 11 unpushed commits, this machine only
- 13 unmerged branches kept (contain work dev lacks): `condors`, `devops`, `equity-calculation-fixes`, `eventticks-dev`, `polygon-refactor`, `queue-refactor-1`, `feature/trade-signals-19-01`, 7 `worktree-agent-*` (worktrees under `.claude/worktrees/`)
- Netting in Simulation: structurally reachable, not enabled (needs shared Simulation Account decision, ADR-0001)

## Queued work
- `.planning/todos/pending/2026-07-04-reconcile-models-and-eventmodels-into-one-package.md` — `src/go/models` vs `src/go/eventmodels`: 47 colliding DIVERGED types; per-type canonical selection + diff testing; includes follow-up `event*`→boring package renames. Do NOT bulk-merge.
- Close `gorm.DB` leaks through `IDatabaseService` (annotated in `data/` code)
- Legacy `LiveAccountType` `mock`/`simulator` values: persisted in DB, need data migration
- `CLAUDE.md` layout section is stale in places (line counts, some paths) — update after next structural change

## Gotchas for the next agent
- GSD is mandatory here unless operator explicitly bypasses (see CLAUDE.md)
- `gh` is aliased to `git checkout` in the user's shell — use `command gh`
- `task test` = Go unit tests; Python: `cd src/clients/python && OTEL_SDK_DISABLED=true ~/miniconda3/envs/grodt/bin/python -m pytest tests/ --ignore=tests/test_strategy_e2e.py`
- Editor diagnostics can be stale during multi-agent edits — trust `go build`, not the diagnostics panel
- `miniconda3` at repo root is a deliberate symlink to `~/miniconda3` — do not delete
- `todo/` at repo root holds tracked architecture docs — not litter
