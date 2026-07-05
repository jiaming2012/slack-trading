## Context

Five Go packages carry names from an abandoned "event-driven" architecture direction (per `CONTEXT.md` and the `next-steps-plan.md` handoff): `eventservices`, `eventconsumers`, `eventproducers`, `eventpubsub`, and the hyphenated container directory `backtester-api`. The pending todo `2026-07-04-reconcile-models-and-eventmodels-into-one-package.md` lists this rename as its own explicit follow-up, done *after* the `models`/`eventmodels` reconciliation so each importing file's import block is rewritten only once. This is a mechanical, behavior-preserving rename — no logic, routing, or data changes.

## Goals / Non-Goals

**Goals:**
- Rename the five packages/directories and every import path that references them, with a package-clause rename to match (`eventservices`→`marketdata`, `eventconsumers`→`workers`, `eventproducers`→`api`, `eventpubsub`→`pubsub`; `backtester-api` directory only, subpackage clauses unchanged).
- Prove zero behavior change via the MIG-03 byte-for-byte diff-test pattern.
- Bring `CLAUDE.md` and the live `.planning/codebase/` docs back in sync with the new names.
- Add a durable, automated guard (Taskfile target) so the docs/code never drift back to the old names.

**Non-Goals:**
- No merge of `models`/`eventmodels` — that is `reconcile-models-packages`, a separate, prerequisite change.
- No rename of `backtester-api`'s subpackages (`models`, `router`, `rpc`, `services`, `playground`, `mock`, `db`, `strategies`) — only the container directory changes.
- No change to import aliases used at call sites (e.g. `backtester_router "..."`) unless a name collision forces one — see Decisions below.
- No functional, RPC, REST, or database schema changes of any kind.
- No changes to `src/clients/python/**`, proto definitions, Docker/Kubernetes manifests, or CI config — none of them reference Go import paths.
- No changes to `.planning/milestones/**`, `.planning/todos/**`, `todo/**`, or `docs/adr/**` — these are historical records of what existed at the time and legitimately keep the old names.

## Decisions

- **Rename order**: directory `mv` + package-clause edit + import-path rewrite, one package at a time, in this order: `eventpubsub` (fewest importers/files, proves the mechanical pattern cheaply) → `eventconsumers` → `eventproducers` → `eventservices` → `backtester-api` (largest, done last once the pattern is proven). Each package gets its own commit so a single bad rename can be reverted without unwinding the others.
- **Tooling**: use `gofmt`/`goimports`-safe find-and-replace (e.g. `grep -rl` to find importers, then `sed -i` on the import path string and the package clause line) rather than a rename-aware refactoring tool, since the scope is a pure string substitution on directory path and one identifier per package — no symbol renames inside package bodies.
- **Import aliases untouched by default**: CLAUDE.md's documented convention aliases `backtester-api/router` as `backtester_router` at import sites to disambiguate from other `router` packages. Since the subpackage clause name (`router`) does not change, existing aliases keep working unchanged and are left as-is to minimize diff noise. Only touch an alias if the rename introduces a genuine new collision (not expected — verified during Task Group 2).
- **Docs scope**: only files that describe *current* live architecture (`CLAUDE.md`, `.planning/codebase/*.md`) are updated. Historical/archival planning documents are explicitly out of scope (see Non-Goals) so the guard command (architecture-doc-consistency capability) excludes those paths.
- **New Taskfile target**: `task lint:package-names` (mechanical grep-based check, no new dependency) — the one new operator-visible command this change introduces, satisfying the project's convention that new commands ship with a Taskfile wrapper in the same change.
- **Diff-test harness reuse**: this change does not build its own MIG-03 harness from scratch — it reuses the fixed backtest scenario and Wave-0 reference capture established by `reconcile-models-packages` (which runs immediately before this change in the batch). Because this change is a pure rename, the expected result is zero diff against that same reference capture.

## Data Flow

No data flow changes. Request/response shapes, Twirp RPC contracts, REST routes, database schema, and the Playground/Broker/Feed/Tick execution path (per `CONTEXT.md`) are all untouched. The only "flow" affected is the Go compiler's import resolution, which is mechanically remapped from old to new paths.

## File / Package Layout (old → new)

| Old | New | Package clause |
|---|---|---|
| `src/go/eventservices/` | `src/go/marketdata/` | `eventservices` → `marketdata` |
| `src/go/eventconsumers/` | `src/go/workers/` | `eventconsumers` → `workers` |
| `src/go/eventproducers/` | `src/go/api/` | `eventproducers` → `api` |
| `src/go/eventpubsub/` | `src/go/pubsub/` | `eventpubsub` → `pubsub` |
| `src/go/backtester-api/` | `src/go/backtester/` | directory only; `models`/`router`/`rpc`/`services`/`playground`/`mock`/`db`/`strategies` clauses unchanged |

## Out of Scope

- `models`/`eventmodels` package merge (prerequisite change, not this one).
- Crossed-enum → mode-preset migration (`migrate-crossed-enums`, a later batch change).
- Any behavior, routing, schema, or dependency change.
- Renaming `backtester-api`'s subpackages.
- Editing historical `.planning/milestones/**`, `.planning/todos/**`, `todo/**`, `docs/adr/**` content.

## Dependency Ordering on Other Batch Changes

- **Hard prerequisite**: `reconcile-models-packages` must land (be merged and archived) before this change starts. It merges `src/go/models` + `src/go/eventmodels` into one `models` package and rewrites all of their importers first — doing this rename afterward means each importing file's import block is edited once, not twice.
- **If `reconcile-models-packages` fails and is reverted** (per the overnight repair policy), this change is blocked and must not proceed against the pre-reconciliation tree — doing so would double the import-rewrite churn the sequencing was designed to avoid.
- **Downstream**: `close-gorm-db-leaks`, `migrate-crossed-enums`, and later Phase 3 greenfield changes are sequenced after this one in the overnight plan; none of them are prerequisites *for* this change.

## Verification Gates

- **G1** `go build ./src/go/... ./cmd/...` green after all five renames land.
- **G2** `task test` green — no new failures versus the Wave-0 baseline.
- **G4** MIG-03 diff-test: the fixed backtest scenario's output is byte-for-byte identical to the Wave-0 reference capture (reused from `reconcile-models-packages`) — expected outcome is a zero-line diff, since this change alters no behavior.
- **G6** Fable adversarial review: since this change is Sonnet-authored, an explicit orchestrator review pass approves the diff (directory moves, import rewrites, doc updates, new Taskfile target) before commit.

## Deferred Validation

None. This is a Tier A card in the overnight plan — build, test, diff-test, and review all run synchronously within this repo with no external dependency (no live broker, no prod DB, no new infra), so no validation is deferred to a later change.
