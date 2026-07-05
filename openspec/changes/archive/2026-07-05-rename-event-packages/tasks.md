## 1. Pre-flight

- [x] 1.1 Confirm `reconcile-models-packages` has landed and been archived on the working branch; if not, stop and do not proceed with this change.
- [x] 1.2 Confirm `go build ./src/go/... ./cmd/...` and `task test` are green at HEAD before starting.
- [x] 1.3 Confirm (or re-run, if not already reused from `reconcile-models-packages`) the MIG-03 reference capture for the fixed backtest scenario — this is the baseline the post-rename diff-test compares against.

## 2. Package renames (one commit per package, smallest to largest)

- [x] 2.1 Rename `src/go/eventpubsub` → `src/go/pubsub`; change `package eventpubsub` → `package pubsub` in every file; rewrite every importer's import path across `src/go/**` and `cmd/**`.
- [x] 2.2 Rename `src/go/eventconsumers` → `src/go/workers`; change `package eventconsumers` → `package workers` in every file; rewrite every importer's import path.
- [x] 2.3 Rename `src/go/eventproducers` → `src/go/api`; change `package eventproducers` → `package api` in every file; rewrite every importer's import path.
- [x] 2.4 Rename `src/go/eventservices` → `src/go/marketdata`; change `package eventservices` → `package marketdata` in every file; rewrite every importer's import path.
- [x] 2.5 Rename directory `src/go/backtester-api` → `src/go/backtester`; rewrite every importer's import path prefix for each subpackage (`models`, `router`, `rpc`, `services`, `playground`, `mock`, `db`, `strategies`); confirm no subpackage's `package` clause changed.
- [x] 2.6 Verify no import alias collisions were introduced by the `backtester-api` → `backtester` rename (e.g. `backtester_router` alias still resolves cleanly); adjust only if a genuine new collision exists.

## 3. Codebase-wide sweep

- [x] 3.1 Run `grep -rl "src/go/eventservices\|src/go/eventconsumers\|src/go/eventproducers\|src/go/eventpubsub\|src/go/backtester-api" --include="*.go" src cmd` and confirm zero results; fix any stragglers.
- [x] 3.2 Run `gofmt -l src/go cmd` (or `go vet ./...`) to confirm no formatting or vet issues from the mechanical edits.

## 4. Documentation sync

- [x] 4.1 Update `CLAUDE.md`'s "Layout" section to the new package names/paths.
- [x] 4.2 Update `CLAUDE.md`'s "Key Subsystems", "Module Design", "Entry Points", and "Conventions" prose wherever it names the old packages.
- [x] 4.3 Update `.planning/codebase/STRUCTURE.md`, `ARCHITECTURE.md`, `CONCERNS.md`, `INTEGRATIONS.md`, `TESTING.md`, and `CONVENTIONS.md` wherever they name the old packages.
- [x] 4.4 Update `taskfile.yml` task descriptions/comments that name the old packages (e.g. the `task test:integration` comment referencing `eventservices`).

## 5. Stale-reference guard tooling

- [x] 5.1 Add a `lint:package-names` target to `taskfile.yml` that greps `src/go`, `cmd`, `CLAUDE.md`, and the six `.planning/codebase/*.md` files listed above for the five old package names, excluding `.planning/milestones/**`, `.planning/todos/**`, `openspec/changes/*/`, `todo/**`, and `docs/adr/**`.
- [x] 5.2 Confirm the target exits 0 with a "no stale references found" message on the clean, fully-renamed tree.
- [x] 5.3 Confirm the target exits non-zero and lists offending file paths when temporarily pointed at a fixture/dirty input (manual sanity check, then revert the fixture — no permanent test fixture needed for a one-shot grep wrapper).

## 6. Verification and closeout

- [x] 6.1 G1: run `go build ./src/go/... ./cmd/...` and confirm it exits 0.
- [x] 6.2 G2: run `task test` and confirm it exits 0 with no new failures versus the Wave-0 baseline.
- [x] 6.3 G4: re-run the MIG-03 fixed backtest scenario and confirm its output is byte-for-byte identical to the Wave-0 reference capture (zero diff lines).
- [x] 6.4 G6: request the Fable adversarial review pass on the full diff (renames, import rewrites, doc updates, new Taskfile target) and obtain explicit approval before the final commit.
- [x] 6.5 Run `task lint:package-names` one last time on the final tree and confirm it passes.
- [x] 6.6 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time).
