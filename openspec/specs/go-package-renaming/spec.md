# go-package-renaming Specification

## Purpose
TBD - created by archiving change rename-event-packages. Update Purpose after archive.
## Requirements
### Requirement: Market Data Package Rename
The system SHALL rename `src/go/eventservices` to `src/go/marketdata`, changing the package clause in every file of that package from `package eventservices` to `package marketdata`, with no other code changes to those files.

#### Scenario: eventservices directory and package clause renamed
- **WHEN** the repository is inspected after this change lands
- **THEN** `src/go/eventservices/` does not exist, `src/go/marketdata/` exists, and every `.go` file under it declares `package marketdata`

### Requirement: Worker Package Rename
The system SHALL rename `src/go/eventconsumers` to `src/go/workers`, changing the package clause in every file of that package from `package eventconsumers` to `package workers`, with no other code changes to those files.

#### Scenario: eventconsumers directory and package clause renamed
- **WHEN** the repository is inspected after this change lands
- **THEN** `src/go/eventconsumers/` does not exist, `src/go/workers/` exists, and every `.go` file under it declares `package workers`

### Requirement: API Package Rename
The system SHALL rename `src/go/eventproducers` to `src/go/api`, changing the package clause in every file of that package from `package eventproducers` to `package api`, with no other code changes to those files.

#### Scenario: eventproducers directory and package clause renamed
- **WHEN** the repository is inspected after this change lands
- **THEN** `src/go/eventproducers/` does not exist, `src/go/api/` exists (including its sub-directories per API domain), and every `.go` file directly under it declares `package api`

### Requirement: Pub/Sub Package Rename
The system SHALL rename `src/go/eventpubsub` to `src/go/pubsub`, changing the package clause in every file of that package from `package eventpubsub` to `package pubsub`, with no other code changes to those files.

#### Scenario: eventpubsub directory and package clause renamed
- **WHEN** the repository is inspected after this change lands
- **THEN** `src/go/eventpubsub/` does not exist, `src/go/pubsub/` exists, and every `.go` file under it declares `package pubsub`

### Requirement: Backtester Directory Rename
The system SHALL rename the container directory `src/go/backtester-api` to `src/go/backtester`. Because `backtester-api` holds no `.go` files of its own (only subpackages `models`, `router`, `rpc`, `services`, `playground`, `mock`, `db`, `strategies`), this rename SHALL change only the import path prefix of each subpackage, not any subpackage's package clause name.

#### Scenario: backtester-api directory renamed, subpackage clauses untouched
- **WHEN** the repository is inspected after this change lands
- **THEN** `src/go/backtester-api/` does not exist, `src/go/backtester/` exists with the same subpackage directories (`models`, `router`, `rpc`, `services`, `playground`, `mock`, `db`, `strategies`), and each subpackage's `.go` files still declare their original package clause (`package models`, `package router`, `package rpc`, `package services`, `package playground`, `package mock`, `package db`, `package strategies`)

### Requirement: Import Path Rewrite Completeness
The system SHALL rewrite every import statement across `src/go/**` and `cmd/**` that references any of the five old import paths (`.../src/go/eventservices`, `.../src/go/eventconsumers`, `.../src/go/eventproducers`, `.../src/go/eventpubsub`, `.../src/go/backtester-api...`) to the corresponding new path, leaving zero references to the old paths anywhere in buildable Go source.

#### Scenario: no importer references an old import path
- **WHEN** `grep -rl "src/go/eventservices\|src/go/eventconsumers\|src/go/eventproducers\|src/go/eventpubsub\|src/go/backtester-api" --include="*.go" src cmd` is run after this change lands
- **THEN** the command returns zero matching files

### Requirement: Behavior Parity Under Rename
The rename SHALL NOT alter runtime behavior. A fixed backtest scenario run through the MIG-03 diff-test harness (established by `reconcile-models-packages`) SHALL produce output byte-for-byte identical to the reference capture taken before this change's rename commits.

#### Scenario: fixed backtest scenario output is unchanged
- **WHEN** the MIG-03 reference backtest scenario is re-run after this change's rename commits land
- **THEN** its output is byte-for-byte identical to the pre-rename reference capture, with zero diff lines

### Requirement: Build and Test Regression Gate
After the rename, the codebase SHALL build cleanly and the existing unit test suite SHALL pass with no new failures introduced by the rename.

#### Scenario: go build succeeds after rename
- **WHEN** `go build ./src/go/... ./cmd/...` is run after this change lands
- **THEN** it exits 0 with no compilation errors

#### Scenario: unit tests pass after rename
- **WHEN** `task test` is run after this change lands
- **THEN** it exits 0 with no test failures that were not already present in the Wave-0 baseline

