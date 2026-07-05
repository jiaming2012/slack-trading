# mode-presets

## ADDED Requirements

### Requirement: Single Mode type with exactly three values

The code SHALL define one `Mode` type whose only valid values are `Simulation`, `Paper`, and `Margin`, each being a named preset that binds a feed source to a broker execution venue per `CONTEXT.md`. A `Validate` (or equivalent) method SHALL accept exactly those three values and reject any other value.

#### Scenario: The three presets validate

- **WHEN** `Mode` is validated against each of `Simulation`, `Paper`, and `Margin`
- **THEN** validation returns no error for all three

#### Scenario: An unknown mode is rejected

- **WHEN** `Mode` is validated against a value that is not one of the three presets (for example `"reconcile"` or `"mock"`)
- **THEN** validation returns a non-nil error

### Requirement: Crossed enums removed from the code surface

The Go types `PlaygroundEnvironment` and `LiveAccountType` SHALL NOT be declared or referenced anywhere in `src/go/**` non-test source after this change, and the `Meta` playground metadata SHALL carry a single `Mode` field in place of the former `Environment` + `LiveAccountType` field pair.

#### Scenario: No references to the retired enum types remain

- **WHEN** the `src/go/**` Go sources are searched for the identifiers `PlaygroundEnvironment` and `LiveAccountType`
- **THEN** zero non-test source files contain either identifier

#### Scenario: Playground metadata exposes Mode

- **WHEN** a playground's `Meta` is inspected in code
- **THEN** it has a single `Mode` field and no `Environment` or `LiveAccountType` field

### Requirement: Illegal feed/venue combinations are unrepresentable

Because a `Mode` is a single value rather than a cross-product of two independent enums, the code SHALL make contradictory combinations (such as a live venue paired with a simulator account) impossible to construct through the `Mode` type. Every construction path that formerly took an `(environment, liveAccountType)` pair SHALL take a single `Mode`.

#### Scenario: No construction path accepts an environment/account-type pair

- **WHEN** the playground and account construction functions are inspected
- **THEN** none of them accept both a `PlaygroundEnvironment` and a `LiveAccountType` argument; each takes a single `Mode`

#### Scenario: Whole tree builds after enum removal

- **WHEN** `go build ./src/go/... ./cmd/...` runs against the post-migration tree
- **THEN** the command exits 0 with no compilation errors (verification gate G1)
