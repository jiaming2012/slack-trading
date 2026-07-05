## Why

Five Go package names (`eventservices`, `eventconsumers`, `eventproducers`, `eventpubsub`, `backtester-api`) still carry the "event-driven" naming from an architecture direction the codebase abandoned, misleading readers and AI tooling about what each package actually does; this change renames them to names that match their real responsibilities with zero behavior change.

## What Changes

- Rename `src/go/eventservices` → `src/go/marketdata` (package clause `eventservices` → `marketdata`); update every importer.
- Rename `src/go/eventconsumers` → `src/go/workers` (package clause `eventconsumers` → `workers`); update every importer.
- Rename `src/go/eventproducers` → `src/go/api` (package clause `eventproducers` → `api`); update every importer.
- Rename `src/go/eventpubsub` → `src/go/pubsub` (package clause `eventpubsub` → `pubsub`); update every importer.
- Rename directory `src/go/backtester-api` → `src/go/backtester` (a container directory with no package clause of its own); update the import path prefix for every subpackage under it (`models`, `router`, `rpc`, `services`, `playground`, `mock`, `db`, `strategies`) in every importer. Subpackage clause names themselves are unchanged.
- Update `CLAUDE.md`'s "Layout" section (and the "Key Subsystems", "Module Design", and "Entry Points" prose that names these packages) to the new paths. **(Executed as a task in this change, not during drafting.)**
- Update `.planning/codebase/STRUCTURE.md`, `ARCHITECTURE.md`, `CONCERNS.md`, `INTEGRATIONS.md`, `TESTING.md`, and `CONVENTIONS.md` wherever they name the old packages.
- Update `taskfile.yml` task descriptions/comments that name the old packages (e.g. `task test:integration` comment referencing `eventservices`).
- Add a new Taskfile target that scans live source and docs for stale references to the five old package names, so drift is caught automatically going forward (this change's one new operator-visible command).
- **BREAKING** (internal only): any other in-flight branch or worktree that imports `src/go/eventservices`, `src/go/eventconsumers`, `src/go/eventproducers`, `src/go/eventpubsub`, or `src/go/backtester-api` by their current import paths will fail to build once this change merges, and will need its imports rewritten to the new paths. No externally-facing API, RPC contract, or Python client behavior changes — Python only talks to the Go server over Twirp/REST, never by importing Go packages.
- No functional, RPC, REST, or database behavior changes. This is a pure identifier rename gated by a byte-for-byte diff test.

## Capabilities

### New Capabilities
- `go-package-renaming`: the behavior-preserving rename of the five packages (directories, package clauses, and every importer's import path), verified by build/test parity and a byte-for-byte behavioral diff test.
- `architecture-doc-consistency`: keeping `CLAUDE.md` and `.planning/codebase/` documentation, plus a new automated guard command, in sync with the renamed package names so the docs never drift back to stale identifiers.

### Modified Capabilities
(none — no existing specs in this repo)

## Impact

- **Affected code paths**: `src/go/eventservices/**` (34 files), `src/go/eventconsumers/**` (21 files), `src/go/eventproducers/**` (22 files), `src/go/eventpubsub/**` (4 files), `src/go/backtester-api/**` (115 files across 8 subpackages), plus every importer of these packages across `src/go/**` and `cmd/**` (24+ distinct importing files by current grep count, some importing more than one of the five).
- **Docs affected**: `CLAUDE.md` (Layout, Key Subsystems, Module Design, Entry Points, Conventions sections), `.planning/codebase/{STRUCTURE,ARCHITECTURE,CONCERNS,INTEGRATIONS,TESTING,CONVENTIONS}.md`, `taskfile.yml` comments.
- **Not affected**: `src/clients/python/**` (Python never imports Go packages by path — it only calls generated Twirp/protobuf stubs), proto definitions, Kubernetes/Docker/deploy manifests, database schema.
- **Dependency on other changes in this batch**: hard dependency on `reconcile-models-packages` landing and being archived first. That change merges `src/go/models` and `src/go/eventmodels` into one `models` package and rewrites all of their importers; sequencing this rename after it means each importing file's import block is rewritten once instead of twice (once for the models merge, once for this rename). If `reconcile-models-packages` fails and is reverted per the overnight repair policy, this change is blocked and should not proceed until it lands.
