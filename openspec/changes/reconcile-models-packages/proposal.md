# Reconcile models and eventmodels into one package

## Why

Two packages — `src/go/models` (37 files) and `src/go/eventmodels` (268 files) — hold overlapping domain types with 47 colliding names that have quietly diverged, so "models" means three different things and both humans and AI tooling pick the wrong copy. This change collapses them into one authoritative `models` package as the foundation the whole refactor track depends on.

## What Changes

- Classify all 47 colliding type names (`Trade`, `Account`, `Candle`, `Strategy`, `PriceLevel`, `SignalV2`, ...) as live-path (backtester, live trading) vs legacy-only (old Slack framework) and record the decision in a committed inventory artifact.
- Pick one canonical copy per colliding type; port every unique exported method from the non-canonical copy or record it as intentionally dropped.
- **BREAKING**: Merge all `src/go/eventmodels` types into package `models` at `src/go/models` and delete the `eventmodels` package. The import path `github.com/jiaming2012/slack-trading/src/go/eventmodels` ceases to exist.
- **BREAKING**: Rewrite imports in all 29 external importers of `src/go/models` and every importer of `eventmodels`, and collapse the internal `eventmodels`→`models` edge (6 files, e.g. `bottraderequest.go`) via co-location.
- Add a fixed-scenario diff-test harness plus a `task` target that compares post-merge backtest output byte-for-byte against a baseline reference captured before the merge (MIG-03 pattern).

## Capabilities

### New Capabilities

- `model-package-inventory` — the committed classification of all 47 colliding types and per-type canonical selection with method-preservation accounting.
- `model-package-consolidation` — the single merged `models` package, deletion of `eventmodels`, and complete import rewrite.
- `model-migration-diff-test` — the baseline capture, byte-for-byte diff-test harness, `task` target, and Python regression parity gate.

### Modified Capabilities

_None — this repo has no existing specs; every capability here is new._

## Impact

- **Code paths**: `src/go/models/` (canonical package after merge), `src/go/eventmodels/` (deleted), 29 external importers of `src/go/models`, all importers of `eventmodels`, the 6 `eventmodels` files that import `models`. `backtester-api/models` is a distinct package and is out of scope.
- **New artifacts**: an inventory/canonical-selection file and a diff-test fixture (baseline reference output) committed under the change or repo test tree, plus one new `taskfile.yml` target.
- **Batch dependencies**: This is the riskiest change of the overnight batch and the root of the refactor track. `rename-event-packages`, `close-gorm-db-leaks`, and `migrate-crossed-enums` all rebase on top of the consolidated package. If this change's gates fail and it is reverted, the entire refactor track halts and the v4 greenfield track proceeds independently on the pre-merge HEAD.
- **Verification**: gated by G1 (`go build`), G2 (`task test`), G3 (pytest failure-set subset of baseline), G4 (MIG-03 byte-for-byte diff-test).
