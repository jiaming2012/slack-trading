# Tasks — reconcile-models-packages

## 1. Baseline capture and diff-test harness

- [x] 1.1 Confirm the pre-merge baseline is green: `go build ./src/go/... ./cmd/...` and `task test` both pass at HEAD; record the commit SHA.
- [x] 1.2 Record the Wave-0 Python baseline failure set: run the pytest suite, capture the exact failing test IDs (expected ~179), commit the list as the parity-gate reference.
- [ ] 1.3 Define the fixed diff-test scenario (AAPL 2025 dates, deterministic strategy config) and record its inputs alongside the fixture.
- [ ] 1.4 Run the fixed scenario at the baseline HEAD and commit its serialized output as the byte-for-byte reference fixture.
- [ ] 1.5 Add a `taskfile.yml` target (e.g. `test:model-diff`) that runs the fixed scenario against the current tree and diffs its output byte-for-byte against the reference, exiting non-zero and naming the scenario on any difference.

## 2. Inventory and canonical selection

- [ ] 2.1 Enumerate all type names declared in `src/go/models` and `src/go/eventmodels`; compute the colliding set (expected 47) and classify each pair as `identical` | `same-name-diverged` | `different-file`.
- [ ] 2.2 For each colliding type, determine live-path vs legacy-only usage and designate exactly one canonical copy.
- [ ] 2.3 For each colliding type, diff the non-canonical copy's exported methods against the canonical copy and record each missing method as `ported` or `dropped-legacy` with a reason.
- [ ] 2.4 Commit the inventory artifact (one row per colliding type: source files, divergence class, canonical choice, per-method accounting, rationale) — this is the `model-package-inventory` deliverable.

## 3. Merge and import rewrite

- [ ] 3.1 In a worktree, move every `eventmodels` type into `package models` at `src/go/models/`, applying the canonical selection and porting methods per the inventory; use non-colliding `snake_case.go` filenames.
- [ ] 3.2 Delete the `src/go/eventmodels` directory so no `eventmodels` Go package remains.
- [ ] 3.3 Collapse the internal `eventmodels`→`models` edge in the 6 affected files (incl. `bottraderequest.go`) by co-location — remove the now-self import.
- [ ] 3.4 Rewrite imports in all 29 external importers of `src/go/models` and every former importer of `eventmodels` to the single consolidated `models` path.
- [ ] 3.5 Grep the tree to confirm no `.go` source file contains `src/go/eventmodels`; resolve any stragglers.

## 4. Verification and closeout

- [ ] 4.1 **G1** — `go build ./src/go/... ./cmd/...` exits 0.
- [ ] 4.2 **G2** — `task test` (Go unit tests) green.
- [ ] 4.3 **G3** — pytest failing-test-ID set is a subset of the committed Wave-0 baseline (no NEW failures).
- [ ] 4.4 **G4** — `task test:model-diff` reports a byte-for-byte match against the baseline reference.
- [ ] 4.5 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time).
