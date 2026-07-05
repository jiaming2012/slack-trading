# model-package-inventory

## ADDED Requirements

### Requirement: Complete colliding-type inventory

The change SHALL produce a committed inventory artifact that lists every type name declared in BOTH `src/go/models` and `src/go/eventmodels`. For each colliding type the inventory SHALL record: the two source file paths, a divergence classification (`identical` | `same-name-diverged` | `different-file`), the canonical choice (`models` | `eventmodels`), and a live-path-vs-legacy-only rationale. The inventory SHALL cover the collision set completely with no omissions and no duplicate entries.

#### Scenario: Every collision is accounted for

- **WHEN** the set of type names declared in both packages is computed from the source at the pre-merge baseline and compared against the inventory artifact
- **THEN** every colliding type name appears in the inventory exactly once, each with a non-empty canonical choice of either `models` or `eventmodels`

#### Scenario: Divergence classification is grounded

- **WHEN** a reviewer inspects any inventory row classified `identical`
- **THEN** the two named source files are byte-for-byte equal, and rows classified `same-name-diverged` name two same-named files whose contents differ

### Requirement: Deterministic canonical selection with method preservation

For each colliding type the inventory SHALL designate exactly one canonical copy, and every exported method present on the non-canonical copy but absent from the canonical copy SHALL be recorded as either `ported` (merged into the canonical type) or `dropped-legacy` (intentionally removed with a stated reason). No exported method SHALL be silently lost.

#### Scenario: No exported method silently lost

- **WHEN** each non-canonical copy's exported method set is diffed against the canonical type's post-merge exported method set
- **THEN** every exported method missing from the canonical type is listed in the inventory as `ported` or `dropped-legacy` with a reason, and none is unaccounted for

#### Scenario: Exactly one canonical per type

- **WHEN** the inventory rows are grouped by colliding type name
- **THEN** each type name has exactly one row and exactly one designated canonical copy
