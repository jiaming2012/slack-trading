# Single market-data provider: Massive; the broker is never a data source

All market data — candles, quotes, option chains — comes from one provider, Massive (formerly Polygon.io), in every mode. Tradier is execution-only. This keeps the data a strategy sees identical across Simulation, Paper, and Margin, which the mode-blind client guarantee (ADR-0002) depends on: if live pricing came from broker quotes while simulation replayed Massive data, the same strategy would see systematically different inputs per mode.

## Consequences

- Existing live paths that price options from Tradier quotes (`eventservices` Tradier quote fetching feeding the option chain map) are declared migration debt.
- Fill realism in Paper mode is bounded by the Tradier sandbox, whose fills may not agree with Massive quotes — an accepted, known gap rather than a bug.
