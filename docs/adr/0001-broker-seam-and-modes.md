# One Broker seam; modes are named (feed, broker) presets

Every order in every mode routes through a single Broker interface. Three adapters satisfy it: the Simulated Broker (our fill engine), the Tradier sandbox, and the Tradier production account. A mode — Simulation, Paper, Margin — is nothing more than a named preset binding a feed source to a broker adapter, selected by environment variables at playground creation. Each adapter manages its own kind of broker account — Simulation Account, Paper Account, Margin Account — so an account for playgrounds to share exists in every mode. Reconciliation (netting logical playground positions into the shared account) lives *behind* the seam and applies in every mode, so Simulation exercises the same netting code paths Margin relies on.

## Considered Options

- **Separate code paths per environment** (status quo): `PlaygroundEnvironment` × `LiveAccountType` crossed enums allow contradictory combinations ("live account of type simulator") and force branching throughout the stack.
- **Netting only for real broker accounts**: cheaper, but leaves the riskiest code path (reconciliation) untested until real money is on the line. Rejected for exactly that reason.

## Consequences

- The Simulated Broker becomes just another adapter — the fill engine must be extracted from the playground tick loop and placed behind the Broker interface.
- Legal mode combinations are fixed by preset names; the crossed-enum model (`PlaygroundEnvironment`, `LiveAccountType`) is legacy to be migrated.
