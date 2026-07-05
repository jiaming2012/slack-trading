## ADDED Requirements

### Requirement: CLAUDE.md Layout Accuracy
`CLAUDE.md` SHALL name only the renamed packages (`marketdata`, `workers`, `api`, `pubsub`, `backtester`) in its "Layout", "Key Subsystems", "Module Design", "Entry Points", and "Conventions" sections, with no remaining references to `eventservices`, `eventconsumers`, `eventproducers`, `eventpubsub`, or `backtester-api`.

#### Scenario: CLAUDE.md contains no stale package names
- **WHEN** `CLAUDE.md` is searched for the strings `eventservices`, `eventconsumers`, `eventproducers`, `eventpubsub`, and `backtester-api` after this change lands
- **THEN** none of those strings appear anywhere in the file

### Requirement: Codebase Reference Docs Accuracy
The live `.planning/codebase/` reference docs (`STRUCTURE.md`, `ARCHITECTURE.md`, `CONCERNS.md`, `INTEGRATIONS.md`, `TESTING.md`, `CONVENTIONS.md`) SHALL name only the renamed packages wherever they currently name the old ones.

#### Scenario: planning codebase docs contain no stale package names
- **WHEN** `grep -rl "eventservices\|eventconsumers\|eventproducers\|eventpubsub\|backtester-api" .planning/codebase/STRUCTURE.md .planning/codebase/ARCHITECTURE.md .planning/codebase/CONCERNS.md .planning/codebase/INTEGRATIONS.md .planning/codebase/TESTING.md .planning/codebase/CONVENTIONS.md` is run after this change lands
- **THEN** the command returns zero matching files

### Requirement: Stale Reference Guard Command
The project SHALL provide an operator-runnable Taskfile target that scans live Go source (`src/go`, `cmd`) and the live docs listed above for the five old package names and fails (non-zero exit) if any are found outside of historical/archival material (`.planning/milestones/**`, `.planning/todos/**`, `openspec/changes/*/`, `todo/**`, `docs/adr/**`), so future drift back to stale names is caught automatically rather than silently accumulating.

#### Scenario: guard passes on a clean tree
- **WHEN** the operator runs the new Taskfile target on a tree with no stale references outside historical material
- **THEN** it exits 0 and reports no stale references found

#### Scenario: guard fails and lists offending files on a dirty tree
- **WHEN** the operator runs the new Taskfile target on a tree where a live file (e.g. `CLAUDE.md` or a `src/go/**` file) still contains one of the five old package names
- **THEN** it exits non-zero and lists the offending file path(s) and matched string(s) in its output
