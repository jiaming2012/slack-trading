# Tasks — migrate-crossed-enums

> Prerequisite: `reconcile-models-packages` and `rename-event-packages` have landed and
> their gates are green. Do not start against a half-migrated tree.

## 1. Mode type

- [x] 1.1 Add `src/go/backtester-api/models/mode.go`: `type Mode string`, consts
      `Simulation` / `Paper` / `Margin`, and `Validate()` accepting exactly those three.
- [x] 1.2 Add a table-driven unit test asserting the three values validate and any other
      value (including `"reconcile"`, `"mock"`) returns an error.

## 2. Boundary compatibility mapping

- [x] 2.1 Add `src/go/backtester-api/models/mode_compat.go` with
      `ModeFromLegacy(environment, liveAccountType string) (Mode, error)` implementing
      the design.md mapping table, returning an explicit error naming offending values
      for unrecognized combinations.
- [x] 2.2 Add `(Mode).ToLegacy() (environment, liveAccountType string)` returning the
      canonical legacy pair per the table.
- [x] 2.3 Unit test: every mapping-table combination resolves to the expected `Mode`;
      round-trip `ModeFromLegacy(ToLegacy(m)) == m` for all three modes; unrecognized
      combination errors.

## 3. Replace crossed enums in the domain model

- [x] 3.1 Collapse `Meta.Environment` + `Meta.LiveAccountType` into a single in-memory
      `Meta.Mode`; keep the `environment` / `live_account_type` GORM columns unchanged
      by serializing through `ToLegacy` / `ModeFromLegacy`.
- [x] 3.2 Update every construction path (playground, account, broker) to take a single
      `Mode` instead of an `(environment, liveAccountType)` pair.
- [x] 3.3 Delete `playground_environment.go` and `live_account_type.go`; rewrite all
      references across the 38 files to `Mode`.
- [x] 3.4 `grep` confirms no non-test `src/go/**` source references
      `PlaygroundEnvironment` or `LiveAccountType`.

## 4. Persistence boundary

- [x] 4.1 In the playground/account stores under `src/go/data/`, call `ModeFromLegacy`
      on the read path after scanning the two columns and `ToLegacy` on the write path.
- [x] 4.2 Confirm no schema migration, backfill, or bulk `UPDATE` against playground
      rows is introduced (diff review).
- [x] 4.3 Unit test with fixture rows for each legacy combination (including a `mock`
      row) verifying read → `Mode` and round-trip write → re-read idempotence.

## 5. RPC boundary

- [x] 5.1 In `CreatePlayground` (`router/grpc.go` + `proto_converters.go`), map request
      `environment` / `live_account_type` to `Mode` via `ModeFromLegacy` and reject
      combinations that do not resolve.
- [x] 5.2 Unit test: a valid `(live, paper)` request resolves to `Paper`; a
      contradictory `(live, simulator)` request is rejected with no playground created.

## 6. Retire operator-facing reconcile concept

- [x] 6.1 Remove any operator entry point that creates/selects a reconcile playground;
      keep the internal reconciliation adapter behind the Broker seam.
- [x] 6.2 Ensure legacy `environment="reconcile"` rows load into the internal
      reconciliation path without panicking; add a load test for such a row.
- [x] 6.3 Confirm `Mode` enumeration contains no reconcile value.

## 7. Verification task target

- [x] 7.1 Add a `taskfile.yml` target (e.g. `test:migrate-crossed-enums`) that runs the
      mode round-trip / diff-test (MIG-03 pattern) and exits non-zero on any mismatch.

## 8. Verification and closeout

- [ ] 8.1 G1 — `go build ./src/go/... ./cmd/...` exits 0.
- [ ] 8.2 G2 — `task test` green (reconciliation/netting unregressed).
- [ ] 8.3 G3 — Python `pytest` failure set is a subset of the Wave-0 baseline (no NEW failures).
- [ ] 8.4 G4 — MIG-03 mode round-trip / diff-test passes via the new `task` target.
- [ ] 8.5 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh
      ROADMAP.md current-state (done at archive time).
