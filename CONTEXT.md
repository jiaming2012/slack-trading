# slack-trading (grodt)

A platform for simulating and executing stock and options trading strategies against external market data feeds. Strategies run identically across historical replay, paper, and real-money execution.

## Language

### Trading sessions

**Playground**:
An isolated strategy-execution container with its own logical positions, orders, and P&L. Multiple playgrounds run concurrently without knowledge of each other, even when they share one broker account.
_Avoid_: book, session

**Logical Position**:
A position as a single playground sees it. Logical positions survive broker-level netting: a playground stays long even while the shared broker account nets to flat or short.

**Broker Account**:
The account behind the Broker seam that holds the NET of all logical positions across the playgrounds sharing it. Three kinds exist, one per mode: Simulation Account, Paper Account, Margin Account.
_Avoid_: live account (overloaded with the `LiveAccountType` code enum)

**Simulation Account**:
An account inside the Simulated Broker holding fake money; our fill engine decides fills. The venue Simulation mode binds to — it exists so netting and reconciliation behave identically in Simulation.
_Avoid_: mock account, simulator account (legacy `LiveAccountType` values)

**Paper Account**:
A broker sandbox account holding fake money; the broker decides mock fills. The venue Paper mode binds to.
_Avoid_: sandbox account, demo account

**Margin Account**:
A real-money margin account at the broker. The venue Margin mode binds to.
_Avoid_: production account, real account

**Reconciliation**:
The netting translation layer between logical positions and the shared broker account. When one playground's order opposes another playground's logical position, reconciliation first sends the broker an offsetting close, then the new order — logical positions are never touched. Internal mechanism; the operator does not create or select it.
_Avoid_: reconcile playground (as an operator-facing concept)

### Modes

**Mode**:
The operator-selected pairing of a market-data source and an execution venue, fixed at playground creation via environment variables. Exactly three exist: Simulation, Paper, Margin.
_Avoid_: environment (overloaded with the deployment sense and the legacy `PlaygroundEnvironment` enum)

**Simulation**:
The mode binding a historical replay feed to a Simulation Account. Runs faster than real time against recorded data.
_Avoid_: backtest vs. simulation as distinct terms — they are the same mode

**Paper**:
The mode binding the real-time feed to a Paper Account — real broker API traffic, mock fills, no real money.
_Avoid_: live-sim, sandbox mode

**Margin**:
The mode binding the real-time feed to a Margin Account — real order execution, real money. As a mode name it refers to the venue only; margin as collateral (free margin, required margin) keeps its ordinary financial meaning.
_Avoid_: production, live (ambiguous — "live" sometimes means real-time data, sometimes real money)

### Execution & data

**Broker**:
The single seam through which every order in every mode is placed and filled. Who sits behind it (Simulated Broker, broker sandbox, real broker) is the mode's choice, invisible to strategies.

**Simulated Broker**:
Our internal fill engine: decides fill price and timing from feed data. The execution venue for Simulation mode.
_Avoid_: mock broker, fill simulator, simulateTick (implementation name)

**Tick**:
The unit of strategy advancement: one request that yields the next candle(s) and any new fills, waiting until they exist. The same contract in every mode — only the wait differs.
_Avoid_: NextTick (implementation name), poll

**Feed**:
The single source of all market data — candles, quotes, and option chains — provided by Massive (formerly Polygon.io) in every mode. The broker is never a data source.
_Avoid_: polygon (stale brand), data source (vague)

### Observability

**Telemetry**:
The platform's own recording of operational signals — metrics, heartbeats, and alerts. Lives inside the trading server, not in an external stack; every recorded number is queryable directly in the platform's database. Emitted in every mode, tagged by mode.
_Avoid_: OTel, observability stack, metrics service (it is a module, not a service)

**Heartbeat**:
A periodic liveness signal emitted by each strategy and each datasource. A heartbeat that stops arriving within its threshold is stale, which raises an Alert. The server does not heartbeat to itself — its death is visible only to the operator.
_Avoid_: ping, health check

**Alert**:
A notification pushed to the operator (via Slack) when a rule over telemetry crosses its threshold — stale heartbeats and error-rate spikes. Alerts are pushed by the server itself; there is no external alerting system.
_Avoid_: alarm, page
