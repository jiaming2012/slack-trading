## ADDED Requirements

### Requirement: Retention Threshold Detection
The system SHALL identify, for `scan_results` and `sim_outcomes`, every partition whose entire date range ends before `now() - retention_window` (default retention window: 12 months) as eligible for archival. Partitions whose range extends into the retention window SHALL NOT be flagged.

#### Scenario: partition older than the retention window is flagged eligible
- **WHEN** the retention window is 12 months and a partition covers a calendar month whose end date is 13 months before now
- **THEN** that partition is included in the eligible-for-archival set

#### Scenario: partition within the retention window is not flagged
- **WHEN** the retention window is 12 months and a partition covers a calendar month whose end date is 11 months before now
- **THEN** that partition is excluded from the eligible-for-archival set

### Requirement: Detach Before Archive
Every partition identified as eligible SHALL be detached from its parent table via `ALTER TABLE ... DETACH PARTITION` before any archival action is taken. Detaching SHALL NOT delete or modify the partition's data — it only removes the partition from the parent's partition set so live queries against the parent table no longer scan or return it.

#### Scenario: detached partition disappears from parent-table queries but data survives
- **WHEN** an eligible partition is detached
- **THEN** querying the parent table (`scan_results` or `sim_outcomes`) for that partition's date range returns zero rows, while querying the detached partition directly by its standalone table name still returns all of its original rows unchanged

### Requirement: Cold-Storage Archive Hook (Stub)
The system SHALL define a `ColdStorageArchiver` interface with a single method invoked exactly once per detached partition, receiving the partition's table name. The shipped implementation SHALL be a stub (`StubArchiver`) that records archival intent locally (e.g. a log line or local manifest entry) and returns success without making any network call or uploading data anywhere. Wiring a real cold-storage backend (e.g. S3/GCS) is explicitly out of scope for this change.

#### Scenario: retention run invokes the stub archiver once per detached partition, no network calls
- **WHEN** the retention job detaches two eligible partitions in a single run
- **THEN** `ColdStorageArchiver.Archive` is invoked exactly twice (once per detached partition) using the stub implementation, the job completes successfully, and no outbound network call is made

### Requirement: Indefinite Retention for Small Analytical Tables
`feature_distributions` and `strategy_ev_weights` SHALL NOT be subject to any retention or archival action introduced by this capability. Rows in these tables are retained indefinitely and are never partitioned, detached, or archived by this job.

#### Scenario: retention job leaves small analytical tables untouched
- **WHEN** the retention job is run
- **THEN** the row counts of `feature_distributions` and `strategy_ev_weights` are unchanged, and neither table is inspected for partitions (they have none) or passed to the archiver

### Requirement: Idempotent, Dry-Run-Capable Retention Job
The retention job SHALL support a dry-run mode that reports which partitions would be detached and archived without performing any detach or archive action. Running the job in live (non-dry-run) mode twice in succession SHALL detach/archive each eligible partition at most once — the second run SHALL find no newly-eligible partitions to act on and SHALL complete without error.

#### Scenario: dry run reports without acting
- **WHEN** the retention job is run in dry-run mode against a database with one eligible partition
- **THEN** the job reports that one partition is eligible for archival, and that partition remains attached to its parent table afterward

#### Scenario: repeated live runs do not double-detach
- **WHEN** the retention job is run in live mode, detaching the one eligible partition, and is then run again in live mode immediately after
- **THEN** the second run detects zero eligible partitions remaining and completes successfully without error or duplicate archiver invocation

### Requirement: Operator-Visible Retention Commands
The system SHALL expose the retention job as operator-runnable Taskfile targets: `db:retention:dry-run` (report only) and `db:retention:run` (detach + archive).

#### Scenario: operator runs the retention commands
- **WHEN** an operator runs `task db:retention:dry-run` followed by `task db:retention:run`
- **THEN** the dry-run prints the eligible-partition report and performs no changes, and the subsequent live run detaches and archives (via the stub) exactly the partitions the dry-run reported
