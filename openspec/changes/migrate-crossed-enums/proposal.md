# Migrate crossed enums to mode presets

## Why

The crossed enums `PlaygroundEnvironment` × `LiveAccountType` allow contradictory, meaningless combinations (a "live account of type simulator") and force mode branching throughout the stack; ADR-0001 declares them legacy, to be replaced by the three named mode presets — Simulation, Paper, Margin — from the `CONTEXT.md` glossary. This change finishes that migration on the code side and retires the operator-facing "reconcile playground" concept, folding reconciliation back to an internal mechanism behind the Broker seam.

## What Changes

- Introduce a single `Mode` type with exactly three values — `Simulation`, `Paper`, `Margin` — as the one operator-selected pairing of feed source and execution venue, so illegal feed/venue combinations become unrepresentable in code.
- **BREAKING**: Replace every code-side use of `PlaygroundEnvironment` and `LiveAccountType` (38 files) with `Mode`; the `Meta.Environment` + `Meta.LiveAccountType` field pair is superseded by a single `Meta.Mode`, and the `PlaygroundEnvironment` / `LiveAccountType` Go types are removed from the code surface.
- **BREAKING**: Retire the operator-facing reconcile playground concept — there is no `Reconcile` mode an operator can select or create; Reconciliation remains an internal netting layer behind the Broker seam per `CONTEXT.md`.
- Add a persistence-boundary compatibility mapping that reads legacy persisted values (`environment` ∈ {simulator, live, reconcile}, `live_account_type` ∈ {mock, simulator, paper, margin, reconcilation}) into `Mode` on load, and writes `Mode` back out using the same legacy string values, so no stored production data is mutated by this change.
- Map the RPC `CreatePlayground` `environment` / `live_account_type` request fields to `Mode` at the router boundary, and reject request combinations that do not correspond to a valid mode preset.
- Add a `task` target that runs the mode round-trip / diff-test verification (MIG-03 pattern) so the operator can confirm the migration is behavior-preserving.

_NON-BREAKING for stored data_: the persisted DB columns and their legacy string values are deliberately left untouched — canonicalizing them is the separate `migrate-live-account-type-data` change, which is never run autonomously.

## Capabilities

### New Capabilities

- `mode-presets` — the single `Mode` type (Simulation, Paper, Margin) that replaces the crossed `PlaygroundEnvironment` × `LiveAccountType` enums in code and makes illegal feed/venue combinations unrepresentable.
- `legacy-enum-compat-mapping` — the persistence- and RPC-boundary compatibility mapping that reads and writes legacy `environment` / `live_account_type` string values as `Mode` without migrating any stored production data, plus the `task` verification target.
- `reconcile-concept-retirement` — removal of the operator-facing reconcile playground concept while preserving the ability to load pre-existing reconcile rows and keeping Reconciliation internal to the Broker seam.

### Modified Capabilities

_None — this repo has no existing specs; every capability here is new._

## Impact

- **Code paths**: all 38 files referencing `LiveAccountType` and every file referencing `PlaygroundEnvironment`, centered on `src/go/backtester-api/models/` (`playground.go`, `playground_meta.go`, `live_account.go`, `live_account_type.go`, `playground_environment.go`, `reconcile_broker.go`, `reconcile_playground.go`, broker/mock files), `src/go/backtester-api/router/grpc.go` and `proto_converters.go`, `src/go/backtester-api/services/`, and `src/go/data/` stores. The persisted GORM columns `environment` and `live_account_type` are read/written unchanged.
- **New artifacts**: a mode round-trip / diff-test fixture and one new `taskfile.yml` target.
- **Batch dependencies**: depends on `reconcile-models-packages` and `rename-event-packages` landing first — the enum types and their importers live in the packages being merged and renamed, so sequencing this change last avoids rewriting the same files twice. If either dependency is reverted, this change rebases onto the pre-refactor HEAD or is skipped.
- **Explicitly out of scope**: mutating the persisted legacy `live_account_type` values (`mock`/`simulator`) in the production Postgres — that is the `migrate-live-account-type-data` change. A real Paper session against the Tradier sandbox is deferred validation (see design.md).
- **Verification**: gated by G1 (`go build`), G2 (`task test`), G3 (pytest failure-set subset of baseline), G4 (MIG-03 mode round-trip / diff-test).
