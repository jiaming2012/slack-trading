# model-package-consolidation Specification

## Purpose
TBD - created by archiving change reconcile-models-packages. Update Purpose after archive.
## Requirements
### Requirement: Single consolidated models package

All domain types formerly declared in `src/go/eventmodels` SHALL reside in package `models` under `src/go/models`, and the `src/go/eventmodels` directory SHALL no longer exist as a Go package. After the merge the codebase SHALL compile with no remaining reference to the `eventmodels` import path.

#### Scenario: eventmodels package no longer exists

- **WHEN** `go list ./src/go/...` is run against the post-merge tree
- **THEN** no package with path `github.com/jiaming2012/slack-trading/src/go/eventmodels` is reported

#### Scenario: Whole tree builds green

- **WHEN** `go build ./src/go/... ./cmd/...` runs against the post-merge tree
- **THEN** the command exits 0 with no compilation errors (verification gate G1)

### Requirement: Complete import rewrite

Every Go source file that imported `src/go/models` or `src/go/eventmodels` SHALL instead import the consolidated `models` package, and no `.go` source file in the repository SHALL contain the string `src/go/eventmodels`.

#### Scenario: No dangling eventmodels imports remain

- **WHEN** the repository `.go` sources are searched for the substring `src/go/eventmodels`
- **THEN** zero source files contain it

#### Scenario: Former models importers still resolve

- **WHEN** `go vet ./src/go/... ./cmd/...` runs against the post-merge tree
- **THEN** it reports no unresolved-import or undefined-symbol errors for the previously-collided type names

### Requirement: No package self-import cycle

The consolidated `models` package SHALL NOT import itself. The former `eventmodels`→`models` dependency edge (6 files including `bottraderequest.go`) SHALL be resolved by co-location, not by an import.

#### Scenario: Internal edge collapsed without a cycle

- **WHEN** the consolidated `models` package is built
- **THEN** `go build` reports no import cycle and the package source contains no import of its own package path

