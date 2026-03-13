# Deprecated Code

This directory contains code that is no longer actively used but is preserved for reference. All code here can also be found in git history.

## Structure

### `go/`
Deprecated Go entry points and tools that are no longer part of the active build.

- **`go/algos/`** — Trendline algorithm (separate go.mod). Originally at `algos/`.
- **`go/src/main.go`** — Alternate entry point that was at `src/main.go`.
- **`go/src/eventticksreader/`** — Event ticks reader. Originally at `src/eventticksreader/`.
- **`go/src/eventstreamviewer/`** — Event stream viewer. Originally at `src/eventstreamviewer/`.
- **`go/src/eventstreamutilities/`** — Event stream utilities. Originally at `src/eventstreamutilities/`.
- **`go/src/cmd/`** — Various CLI tools:
  - `backtester/` — Backtester CLI
  - `backtester-sandbox/` — Backtester sandbox
  - `fetch_market_data/` — Fetch market data tool
  - `fetch_orders/` — Fetch orders tool
  - `import_signals/` — Signal importer
  - `import_trading_view_data/` — TradingView data importer
  - `options_sandbox/` — Options sandbox
  - `pandas_market_calendars/` — Market calendars tool
  - `parse_tradier_orders_csv/` — Tradier CSV parser
  - `sheets/` — Google Sheets integration
  - `trade/` — Trade CLI
  - `server.go` — Server entry point
- **`go/cmd/`** — Top-level sandboxes and tools:
  - `sandbox.go`, `sandbox/` — General sandbox (options, scrap, indicators)
  - `stats-sandbox/` — Stats sandbox
  - `telemetry/` — Telemetry quickstart
- **`go/cmd/stats/`** — Go data processing tools:
  - `clean_data_kaplan_meier/`, `clean_data_pdf/` — Data cleaning
  - `derive_expected_profit/` — Profit derivation
  - `export_data/`, `generate_data/` — Data I/O
  - `plot_candlestick/` — Candlestick charting
  - `transform_data/` — Supertrend strategy data transforms

### `python/`
Deprecated Python scripts.

- **`python/stats/`** — Older Python scripts from `src/cmd/stats/`:
  - Strategy versions v1-v6 (`options_strategy_basic.py` through `v6`, `simple_open_strategy_v1-v3.py`, `simple_stack_open_strategy_v1.py`, `candlestick_open_strategy_v1.py`)
  - Distribution/stats tools (`fit_distribution.py`, `distributions.py`, `kaplan_meier.py`, `pdf.py`, `stationary_test.py`)
  - Expected profit calculators (`derive_expected_profit*.py` variants)
  - Other utilities (`check_positions.py`, `create_indicators.py`, `fetch_options.py`, `market_info.py`, `options_sandbox*.py`, etc.)

- **`python/backtester/`** — PPO/Reinforcement Learning training scripts. Originally at `cmd/backtester/`.
  - `proximal_policy_optimization_v2-v14.py` — PPO training script iterations
  - `validate_model_*.py` — Model validation scripts
  - `backtester_playground_client.py` — Backtester client
  - `requirements.txt` — Python dependencies for PPO training
  - This work may be resumed in the future.
