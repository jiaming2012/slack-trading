# Design — reconcile-models-packages

## Context

`src/go/models` (37 files) and `src/go/eventmodels` (268 files) are the residue of an abandoned event-driven direction. 47 type names collide; measured on the current tree: 6 `eventmodels` files import `models` (`bottraderequest.go`, `balance.go`, `trendspider.go`, `get_strategies_response_event.go`, `addstrategyrequest.go`, `addaccountresponsevent.go`) and 29 external files import `src/go/models`. A third package, `backtester-api/models`, also answers to "models" but is a separate, live package and is explicitly OUT of scope. This is the riskiest change of the overnight batch; the whole refactor track rebases on its output.

## Approach

Three ordered stages, matching the three capabilities:

1. **Inventory + canonical selection + harness first (capability `model-package-inventory`, `model-migration-diff-test` baseline).** Nothing merges until the reference output is captured on the pre-merge baseline and the collision table is complete. This is what makes the merge reversible and checkable — the byte-for-byte diff is the substitute for a human watching.
2. **Merge + import rewrite (capability `model-package-consolidation`).** Move every `eventmodels` type into package `models` at `src/go/models`, porting or dropping methods per the inventory. Delete `src/go/eventmodels`. Rewrite all importers (both packages) to the single path. Collapse the internal `eventmodels`→`models` edge by co-location — the 6 files simply stop importing a sibling that is now the same package.
3. **Verification + cleanup (capability `model-migration-diff-test` gates).** Run G1–G4. On any red gate, follow the batch revert policy: atomic per-change commits make a full revert clean.

## Package / file layout

- Canonical package: `package models` at `src/go/models/` (name chosen because event-driven naming dies with the abandoned direction).
- File naming stays `snake_case.go` per repo convention. Where a colliding type's canonical copy comes from `eventmodels`, its file moves into `src/go/models/` under a non-colliding filename; ported methods from the loser are appended to the canonical type's file.
- Inventory artifact: a committed table (e.g. `openspec/changes/reconcile-models-packages/inventory.md` or a repo-tracked CSV) with one row per colliding type — source files, divergence class, canonical choice, per-method ported/dropped accounting.
- Diff-test fixture: fixed-scenario reference output committed under the repo test tree (e.g. `src/go/backtester-api/testdata/` or a diff-test dir), with recorded scenario inputs (symbol, date range, strategy config). Use AAPL 2025 dates — MSFT 2024 returns Polygon 403 per project gotchas.
- New `taskfile.yml` target (e.g. `task test:model-diff`) runs the scenario against the current tree and diffs byte-for-byte against the fixture. Per project convention, this operator-visible command ships in this same change.

## Data flow (diff-test)

Fixed scenario config → backtester run (Simulation mode, Simulated Broker, replay Feed, deterministic Tick loop) → serialized output → byte-for-byte compare against committed baseline reference → exit 0/non-zero. Determinism holds because Simulation mode replays recorded Feed data with no wall-clock or network variance in the compared output.

## Canonical-selection rule

For each colliding type: if it is exercised on a live code path (backtester, live trading), that copy is canonical; if only the legacy Slack framework uses it (price levels, strategies v1), the legacy copy is canonical only if the live path does not use the type at all. Port unique exported methods from the loser onto the winner; a method used by no live caller and only by deleted/legacy code is recorded `dropped-legacy` with a reason. Every decision lands as an inventory row — no silent choices.

## Out of scope

- `backtester-api/models` is untouched — it is a distinct, live package, not part of the "models" ambiguity being resolved here.
- Package renames (`eventservices`→`marketdata`, `eventconsumers`→`workers`, `eventproducers`→`api`, `eventpubsub`→`pubsub`, `backtester-api`→`backtester`) are a SEPARATE change (`rename-event-packages`) that rebases on this one.
- The crossed-enum → mode-preset migration (`migrate-crossed-enums`) is separate and depends on this change.
- No behavior changes: the merge is purely structural. The byte-for-byte diff-test exists precisely to prove behavior is unchanged.

## Dependency ordering on other batch changes

This change is the root of the refactor track. `rename-event-packages`, `close-gorm-db-leaks`, and `migrate-crossed-enums` all rebase on the consolidated package and MUST land after it. If this change's gates fail and it is reverted, the refactor track halts and the v4 greenfield track proceeds on the pre-merge HEAD (it does not strictly need the consolidation).

## Verification gates

- **G1** — `go build ./src/go/... ./cmd/...` exits 0.
- **G2** — `task test` (Go unit tests) green.
- **G3** — pytest failing-test-ID set is a subset of the committed Wave-0 baseline (179 known failures); gate fails only on a NEW failure.
- **G4** — MIG-03 byte-for-byte diff-test: fixed backtest scenario output identical to the pre-merge baseline reference, via the new `task` target.

## Deferred validation

None deferred — this is a Tier A change. All four gates (G1–G4) are deterministic and run to completion overnight against local fixtures and the local build; no external infra, prod DB, or live/paper broker traffic is required. G3 depends only on the local Python suite and the committed baseline failure list; G4 depends only on committed fixture data (AAPL 2025 replay), not live Polygon calls.

## Overnight constraints honored

No `git push`, no deploys, no prod DB, no Tradier live/paper orders, no new external infra, no force operations. Local Docker (testcontainers, `task db:start`) is permitted but not required by any gate here. Worktree isolation is recommended for the 300+ file bulk edit.
