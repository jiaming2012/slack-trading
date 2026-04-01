---
phase: quick
plan: 260401-gig
type: execute
wave: 1
depends_on: []
files_modified:
  - src/go/backtester-api/router/grpc.go
  - src/clients/python/strategies/mean_reversion_v2.py
autonomous: true
requirements: []
must_haves:
  truths:
    - "WriteSignal RPC writes to globalSignalRepo are queryable via GetProcessedSignals during sim playgrounds"
    - "Python MeanReversionStrategyV2 calls write_signal() on the server when produce_signals() detects a signal"
  artifacts:
    - path: "src/go/backtester-api/router/grpc.go"
      provides: "Sim playground uses shared globalSignalRepo instead of empty InMemorySignalRepository"
      contains: "s.globalSignalRepo"
    - path: "src/clients/python/strategies/mean_reversion_v2.py"
      provides: "Strategy calls playground.write_signal() after signal detection"
      contains: "write_signal"
  key_links:
    - from: "src/clients/python/strategies/mean_reversion_v2.py"
      to: "src/clients/python/engine/client.py"
      via: "self.playground.write_signal() call"
      pattern: "self\\.playground\\.write_signal"
    - from: "src/go/backtester-api/router/grpc.go"
      to: "s.globalSignalRepo"
      via: "SetSignalRepo in CreatePlayground default case"
      pattern: "playground\\.SetSignalRepo\\(s\\.globalSignalRepo\\)"
---

<objective>
Share the Go server's globalSignalRepo with sim playgrounds and wire Python strategies to call write_signal() on signal detection.

Purpose: Currently sim playgrounds get an empty InMemorySignalRepository that nothing writes to, so GetProcessedSignals always returns empty. By sharing globalSignalRepo and having Python strategies call write_signal(), signals become queryable.

Output: Two modified files — one Go line change, one Python write_signal() call addition.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@src/go/backtester-api/router/grpc.go
@src/clients/python/strategies/mean_reversion_v2.py
@src/clients/python/engine/client.py

<interfaces>
<!-- From engine/client.py line 701 — already exists, just needs to be called -->
```python
def write_signal(self, name: str, symbol: str, timestamp: datetime, attributes: dict = None) -> str:
    """Produce a global TradeSignal via WriteSignal RPC. Returns signal_id (UUID string)."""
```

<!-- From base_strategy.py — strategy has access to playground and symbol -->
```python
class BaseStrategy(ABC):
    def __init__(self, playground, symbol: str, logger=None, **kwargs):
        self.playground = playground  # BacktesterPlaygroundClient instance
        self.symbol = symbol          # e.g. "AAPL"
```

<!-- From datasources/ma_crossover.py — produce_signals return format -->
```python
def produce_signals(bar_dict, prev_bar, pdf) -> list:
    """Returns list of dicts with keys: signal_key, bar_dict, pdf_entry."""
```

<!-- From grpc.go line 1507-1514 — current sim default case -->
```go
switch playgroundEnvironment {
case models.PlaygroundEnvironmentLive, models.PlaygroundEnvironmentReconcile:
    if s.esdbProducer != nil {
        playground.SetSignalRepo(models.NewESDBSignalRepository(s.esdbProducer))
    }
default:
    playground.SetSignalRepo(models.NewInMemorySignalRepository())
}
```
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Share globalSignalRepo with sim playgrounds in Go server</name>
  <files>src/go/backtester-api/router/grpc.go</files>
  <action>
In `CreatePlayground` method (around line 1512-1513), change the default case from:
```go
default:
    playground.SetSignalRepo(models.NewInMemorySignalRepository())
```
to:
```go
default:
    playground.SetSignalRepo(s.globalSignalRepo)
```

This is a single-line change. The `s.globalSignalRepo` field already exists on the Server struct and is used by `WriteSignal` (line 1579) and `GetProcessedSignals` (line 1599). Since there is only one playground per simulation, sharing the global repo has no multi-consumer cursor issues.
  </action>
  <verify>
    <automated>cd /Users/jamal/projects/slack-trading && go build ./src/go/backtester-api/...</automated>
  </verify>
  <done>Sim playground default case uses s.globalSignalRepo instead of NewInMemorySignalRepository(). Go builds without errors.</done>
</task>

<task type="auto">
  <name>Task 2: Wire MeanReversionStrategyV2 to call write_signal() on signal detection</name>
  <files>src/clients/python/strategies/mean_reversion_v2.py</files>
  <action>
In `_process_htf_candle()` (line 275), after the `produce_signals()` call and before the `for sig in trade_signals:` loop, add a write_signal() call for each detected signal. Insert after line 281 (`self._prev_htf_bar = bar_dict`), before the `for sig in trade_signals:` loop on line 283:

```python
        # Write signals to server for observability (queryable via GetProcessedSignals)
        for sig in trade_signals:
            try:
                bar_ts = bar_dict.get("datetime", datetime.now())
                if isinstance(bar_ts, str):
                    from dateutil.parser import parse as parse_dt
                    bar_ts = parse_dt(bar_ts)
                self.playground.write_signal(
                    name=sig["signal_key"],
                    symbol=self.symbol,
                    timestamp=bar_ts,
                    attributes={"source": "ma_crossover", "htf_close": str(bar_dict.get("close", ""))},
                )
            except Exception as e:
                self.logger.warning(f"write_signal failed for {sig['signal_key']}: {e}")
```

The existing `for sig in trade_signals:` loop that follows (line 283) continues unchanged — it handles the local strategy logic (pdf_entry evaluation, group creation). The new block only pushes the signal to the server for observability.

Important: The write_signal call is wrapped in try/except so a server RPC failure does not break the strategy's trading logic. Use `self.logger.warning` (not `_default_logger`) since the strategy instance has its own logger.
  </action>
  <verify>
    <automated>cd /Users/jamal/projects/slack-trading/src/clients/python && /Users/jamal/miniconda3/envs/grodt/bin/python -c "import strategies.mean_reversion_v2; print('import OK')"</automated>
  </verify>
  <done>MeanReversionStrategyV2._process_htf_candle() calls self.playground.write_signal() for each detected signal before processing it locally. Import succeeds without errors.</done>
</task>

</tasks>

<verification>
1. Go build passes: `go build ./src/go/backtester-api/...`
2. Python import succeeds: `python -c "import strategies.mean_reversion_v2"`
3. Grep confirms globalSignalRepo wired in default case: `grep -n "globalSignalRepo" src/go/backtester-api/router/grpc.go` shows the new line
4. Grep confirms write_signal called from strategy: `grep -n "write_signal" src/clients/python/strategies/mean_reversion_v2.py`
</verification>

<success_criteria>
- Sim playgrounds share the server's globalSignalRepo so WriteSignal writes are visible to GetProcessedSignals
- MeanReversionStrategyV2 pushes detected signals to the server via write_signal() RPC
- No regression in strategy trading logic (write_signal failure is caught and logged, does not interrupt trading)
</success_criteria>

<output>
After completion, create `.planning/quick/260401-gig-share-globalsignalrepo-with-sim-playgrou/260401-gig-SUMMARY.md`
</output>
