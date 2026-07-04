# Mode-blind clients: no code branches on mode, anywhere

Strategy scripts and shared engine code never branch on which mode (Simulation, Paper, Margin) they run in. The Playground interface absorbs every difference — including time: `tick()` blocks until the next candle is available, returning instantly in Simulation and after a real-time wait in Paper/Margin. Strategies never sleep, poll, or read wall-clock time. Switching modes is done exclusively through environment variables at startup.

## Consequences

- Existing `is_live` branches in the Python engine (e.g. `trading_engine.py` log level and tracing spans) are declared debt and must be pushed behind the Playground interface.
- Cross-cutting concerns that legitimately differ per mode (trace verbosity, span attributes) must be configured at the platform layer, not decided in client code.
- A strategy that passes in Simulation runs unmodified in Margin mode; any behavioral difference between modes is by definition a platform bug, not a strategy bug.
