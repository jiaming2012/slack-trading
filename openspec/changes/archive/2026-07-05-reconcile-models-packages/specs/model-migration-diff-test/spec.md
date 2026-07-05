# model-migration-diff-test

## ADDED Requirements

### Requirement: Baseline reference capture

Before any merge or import rewrite, the change SHALL capture a reference output from a fixed, deterministic backtest scenario run at the pre-merge baseline (Wave-0 HEAD) and commit it as a fixture. The fixture SHALL be reproducible from the same fixed scenario inputs (symbol, date range, strategy config) recorded alongside it.

#### Scenario: Baseline fixture exists and is non-empty

- **WHEN** the diff-test fixture location is inspected after the change lands
- **THEN** a reference output file captured from the fixed scenario exists, is non-empty, and its scenario inputs are recorded next to it

### Requirement: Byte-for-byte diff-test target

The change SHALL add a `taskfile.yml` target that runs the fixed backtest scenario against the current tree and compares its output byte-for-byte against the committed baseline reference, exiting 0 only on an exact match and non-zero on any difference. The target SHALL name the differing scenario when it fails.

#### Scenario: Diff-test passes on identical post-merge output

- **WHEN** the diff-test `task` target runs against the post-merge tree and the scenario output equals the baseline reference
- **THEN** the command exits 0 and reports a byte-for-byte match (verification gate G4)

#### Scenario: Diff-test fails on any drift

- **WHEN** the post-merge scenario output differs from the baseline reference by at least one byte
- **THEN** the `task` target exits non-zero and names the differing scenario

### Requirement: Python regression parity gate

After the merge, the Python test suite's set of failing test IDs SHALL be a subset of the recorded Wave-0 baseline failure list (179 known pre-existing failures). The gate SHALL fail if and only if a test that passed at baseline now fails.

#### Scenario: No new Python failures introduced

- **WHEN** pytest runs against the post-merge tree and its failing test IDs are compared to the committed Wave-0 baseline failure list
- **THEN** every failing test ID is present in the baseline list and the gate passes (verification gate G3)

#### Scenario: A newly-failing test trips the gate

- **WHEN** a test that passed at the Wave-0 baseline fails after the merge
- **THEN** the parity gate reports that test ID as a new failure and exits non-zero
