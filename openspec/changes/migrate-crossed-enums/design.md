# Design — migrate-crossed-enums

## Context

ADR-0001 established one Broker seam with three named modes and declared the crossed
enums `PlaygroundEnvironment` (`simulator|live|reconcile`) × `LiveAccountType`
(`mock|simulator|paper|margin|reconcilation`) as legacy. Today those two enums appear
together across 38 files, allowing nonsensical pairs and forcing two-axis branching.
`CONTEXT.md` fixes the vocabulary: exactly three modes — Simulation, Paper, Margin —
each a named `(feed, broker venue)` preset, with Reconciliation demoted to an internal
mechanism behind the seam.

## Approach

Collapse the two-axis cross-product into a single closed `Mode` enum, then push all
legacy string handling to the two boundaries where the outside world still speaks the
old vocabulary: the Postgres persistence layer and the RPC request surface. Internally,
only `Mode` flows.

### File / package layout

- `src/go/backtester-api/models/mode.go` (new): declares `type Mode string`, the three
  const values, and `Validate()`. Replaces `playground_environment.go` and
  `live_account_type.go`, which are deleted.
- `src/go/backtester-api/models/mode_compat.go` (new): the boundary mapping functions —
  `ModeFromLegacy(environment, liveAccountType string) (Mode, error)` and
  `(Mode).ToLegacy() (environment, liveAccountType string)`. This is the single place
  the legacy strings are known; nothing else references them.
- `src/go/backtester-api/models/playground_meta.go`: `Meta.Environment` +
  `Meta.LiveAccountType` fields collapse to a single in-memory `Mode`. The GORM column
  tags for `environment` and `live_account_type` are preserved (unchanged schema) by
  serializing `Mode` through `ToLegacy()` in the store's read/write path, or via GORM
  hooks — whichever keeps the persisted columns byte-identical.
- `src/go/data/*_store.go`: the read path calls `ModeFromLegacy` after scanning the two
  columns; the write path calls `ToLegacy` before persisting. No schema change.
- `src/go/backtester-api/router/grpc.go` + `proto_converters.go`: `CreatePlayground`
  maps request `environment` / `live_account_type` → `Mode` via `ModeFromLegacy` and
  rejects combinations that do not resolve.
- `reconcile_broker.go` / `reconcile_playground.go`: keep the internal reconciliation
  adapter behind the seam; remove any operator-facing "create reconcile playground"
  entry point. Legacy `environment="reconcile"` rows load into the internal
  reconciliation path, not a `Mode`.

### Data flow

```
Postgres row (environment, live_account_type)  --ModeFromLegacy-->  Mode  (in-memory, everywhere)
Mode  --ToLegacy-->  (environment, live_account_type)  --> Postgres row   (columns unchanged)
RPC CreatePlayground(environment, live_account_type)  --ModeFromLegacy-->  Mode
```

Legacy mapping table (canonical write pair marked `*`):

| environment | live_account_type | Mode        |
|-------------|-------------------|-------------|
| simulator   | mock              | Simulation  |
| simulator   | simulator *       | Simulation  |
| live        | paper *           | Paper       |
| live        | margin *          | Margin      |
| reconcile   | reconcilation     | (internal reconciliation — not a Mode) |

Round-trip is idempotent: read→`Mode`→write→read yields the same `Mode`, even though a
legacy `mock` row is rewritten as `simulator` (both read back as `Simulation`). The
stored *value* of pre-existing rows is not rewritten by this change — rewriting only
happens if/when that row is saved through normal application flow, and even then it maps
to the same `Mode`. No bulk migration is issued.

## Out of scope

- **Mutating persisted legacy values.** The `mock`/`simulator`/`reconcilation` strings
  already in the production DB are NOT canonicalized here. That is the separate
  `migrate-live-account-type-data` change, which is operator-only and never run
  autonomously. This change only reinterprets those values at the boundary.
- **Schema/column changes.** The `environment` and `live_account_type` columns stay.
- **Renaming the packages** the enums live in — that is `rename-event-packages`.
- **Enabling shared Simulation Account netting** — separate ADR-0001 follow-up.
- **Proto field redesign.** The `.proto` keeps its `environment` / `live_account_type`
  string fields; only the Go-side handling changes.

## Dependency ordering on other batch changes

1. `reconcile-models-packages` (merges the packages the enum types live in) — MUST land first.
2. `rename-event-packages` (renames those packages) — MUST land first.
3. `migrate-crossed-enums` (this change) — rebases on top so the 38 enum-referencing
   files are rewritten once, not twice.

If either dependency is reverted, this change rebases onto pre-refactor HEAD or is
skipped for the night; it does not proceed against a half-migrated tree.

## Verification gates

- **G1** `go build ./src/go/... ./cmd/...` green after enum removal.
- **G2** `task test` green — reconciliation/netting behavior unregressed.
- **G3** Python `pytest` failure set is a subset of the Wave-0 baseline (179 known
  pre-existing failures); no NEW failures.
- **G4** MIG-03 mode round-trip / diff-test: every legacy `(environment,
  live_account_type)` combination in the mapping table maps to the expected `Mode` and
  is idempotent on re-read; run via the new `task` target.

## Deferred validation (Tier B)

A real Paper session against the Tradier sandbox is deferred — it needs external broker
connectivity, which overnight constraints prohibit (no live/paper broker orders, no new
external infra). This change lands compile-clean and unit-tested; the Paper-mode
end-to-end confirmation is an operator follow-up. Per the overnight plan the persisted
data migration (`migrate-live-account-type-data`) is also explicitly not attempted.
