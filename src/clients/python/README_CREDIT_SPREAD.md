# Credit Spread Strategy

PDF-guided options strategy that sells credit spreads on mean-reversion signals. Collects premium upfront and profits from time decay when the PDF correctly predicts price reversion.

- **Bullish signal** (stock dipped) -> sell bull put credit spread below current price
- **Bearish signal** (stock rallied) -> sell bear call credit spread above current price

## Quick Start

```bash
# Basic run — hold-to-expiration with 2% OTM buffer, no stop-loss (spread width caps loss)
python demo_credit_spread.py \
  --symbol AAPL \
  --start 2025-06-01 \
  --end 2025-09-15 \
  --retrain-interval weekly

# With additional entry filters for tighter risk
python demo_credit_spread.py \
  --symbol AAPL \
  --start 2025-06-01 \
  --end 2025-09-15 \
  --retrain-interval weekly \
  --min-p-profit 0.70 \
  --min-credit-width-ratio 0.20 \
  --max-open-positions 10 \
  --max-positions-per-expiration 3 \
  --daily-loss-limit 0.05
```

Outputs `spread_decay_curves.html` and `pl_surface.html` visualizations automatically after the run.

## Parameters

### Simulation

| Flag | Default | Description |
|------|---------|-------------|
| `--symbol` | `AAPL` | Underlying stock symbol |
| `--start` | `2025-06-01` | Backtest start date (YYYY-MM-DD) |
| `--end` | `2026-02-28` | Backtest end date (YYYY-MM-DD) |
| `--balance` | `100000` | Starting account balance |
| `--pdf-path` | `""` | Path to pre-built PDF pickle (builds from Polygon if empty) |
| `--model` | `bayesian_nig` | PDF model: `empirical` or `bayesian_nig` |
| `--twirp-host` | `http://127.0.0.1:5051` | Backtester gRPC server address |

### Spread Construction

| Flag | Default | Description |
|------|---------|-------------|
| `--max-spread-width` | `5.0` | Maximum spread width ($). Actual width is `min(PDF-driven, risk-capped, this value)`. Acts as a ceiling. |
| `--spread-width-sigma` | `1.0` | Sigma multiplier for PDF-driven width: `width = stock_price × horizon_stddev × N`. Higher = wider spreads based on the model's expected move. |
| `--max-loss-per-trade` | `0.0` | Max loss budget per trade ($). Caps width via `width = budget / (100 × contracts)`. At 0, disabled (uses PDF and spread-width ceiling only). Example: `500` limits each 5-contract spread to $1.00 width. |
| `--min-credit` | `0.50` | Minimum net credit per spread ($) to enter. Rejects thin spreads that don't justify the risk. |
| `--min-spread-width` | `2.50` | Minimum acceptable actual width between strikes ($). Prevents very narrow spreads with negligible credit. |
| `--total-contracts` | `5` | Number of contracts per spread entry. Each contract = 100 shares of exposure. |
| `--long-only` | `false` | Only trade bullish signals (bull put spreads). Skip all bearish/bear call signals. |

### Option Selection

| Flag | Default | Description |
|------|---------|-------------|
| `--target-dte` | `30` | Target days to expiration when selecting option contracts. Longer DTE = slower theta but more time for the trade to work. |
| `--min-dte` | `14` | Minimum DTE — reject contracts expiring sooner than this. Avoids illiquid near-term options. |
| `--max-dte` | `45` | Maximum DTE — reject contracts expiring later than this. Avoids tying up capital too long. |

### Signal Detection

| Flag | Default | Description |
|------|---------|-------------|
| `--htf-horizon` | `1h` | Higher timeframe candle period for signal detection (e.g. `1h`, `4h`). Signals are detected on this timeframe, entries on 5-min bars. |
| `--stop-percentile` | `0.95` | PDF percentile used to set the stop price for deviation levels. Higher = wider stop = more levels triggered. |
| `--tail-threshold` | `0.005` | Minimum PDF tail probability for auto-generated sigma steps. Filters out levels in extremely unlikely tails. |

### Exit Conditions

Exits are checked in priority order. The first matching condition triggers the exit.

| Flag | Default | Description |
|------|---------|-------------|
| `--profit-target` | `1.0` | Take profit when N% of credit is captured. At 1.0 (default), effectively disabled — the spread must be worth $0 to trigger, which only happens at expiration. Set to 0.50 to exit when 50% captured. |
| `--max-loss-mult` | `2.0` | Exit when spread mark-to-market value exceeds `N * credit_per_contract`. At 2.0, stops at 2x the credit received. Lower = tighter stop. |
| `--time-decay-exit` | `5` | Force exit when DTE drops to N days. Avoids holding into expiration week where gamma risk spikes. |
| `--min-hold-candles` | `12` | Minimum 5-min candles before any exit check runs (12 = 1 hour). Prevents whipsaw exits immediately after entry. |
| `--enable-reversion-exit` | `false` | Exit when price reverts to the signal price. **Off by default** (hold-to-expiration mode). Set this flag to enable the original short-term exit behavior. |

### Risk Management: Entry Gates

These parameters filter trades **before** entry. Set to `0` (or omit) to disable.

| Flag | Default | Description |
|------|---------|-------------|
| `--min-p-profit` | `0.0` | Minimum P(profit) from the PDF model to enter a spread. The model estimates the probability that the stock stays OTM through the horizon. Example: `0.70` requires 70% win probability. At 0, all positive-EV trades are allowed. |
| `--min-credit-width-ratio` | `0.0` | Minimum ratio of net credit to spread width. Ensures the premium collected justifies the defined risk. Example: `0.20` on a $5 spread requires at least $1.00 credit. At 0, any credit above `--min-credit` is accepted. |
| `--max-open-positions` | `0` | Hard cap on total simultaneous open spread entries across all groups. Prevents over-leveraging when positions are held for weeks. At 0, limited only by collateral. |
| `--max-directional-imbalance` | `0` | Maximum excess of bullish over bearish groups (or vice versa). Prevents the portfolio from becoming one-directional. Example: `3` means no more than 3 extra bull groups vs bear groups. At 0, no directional limit. |
| `--max-positions-per-expiration` | `0` | Maximum spreads sharing the same expiration date. Prevents catastrophic loss if all positions expire on the same day and the stock gaps against you. Example: `3` caps at 3 spreads per Friday. At 0, unlimited. |
| `--daily-loss-limit` | `0.0` | Stop entering new trades if intraday equity drawdown exceeds this fraction. Example: `0.05` halts entries after a 5% daily loss. Does not close existing positions — only prevents new ones. At 0, no daily limit. |
| `--min-otm-pct` | `0.02` | Minimum OTM distance for the short strike as a fraction of stock price. At 0.02, the short put must be at least 2% below the stock price (and short call at least 2% above). Prevents placing the short leg ATM where it's immediately at risk. Set to 0 to disable. |
| `--cooldown-candles` | `78` | After exiting a spread, wait N five-minute candles before re-entering the same strikes (78 = 1 trading day). Prevents the churn pattern where the same spread is entered and exited repeatedly. Set to 0 to disable. |

### Risk Management: Position-Level Exits

These exits protect individual positions during the hold. Set to `0` or omit to disable.

| Flag | Default | Description |
|------|---------|-------------|
| `--enable-strike-breach-exit` | `false` | Exit when the underlying's close crosses the short strike. **Off by default** — the spread width already caps max loss, so no stop-loss is needed. Enable only if you want to exit before max loss is realized. |
| `--gamma-risk-dte` | `0` | When DTE drops to N days, exit if the stock is within a buffer zone of the short strike. **Off by default** — trust the model and let the spread expire. Set to 7 to enable near-expiration protection. |
| `--gamma-risk-buffer-pct` | `0.50` | Buffer zone for gamma exit, as a fraction of spread width. At 0.50 on a $5 spread, exits if stock is within $2.50 of the short strike (measured from the profitable side). Wider buffer = more conservative near expiration. |
| `--early-profit-time-pct` | `0.25` | Early profit exit: max fraction of holding period elapsed. If more than N% of the expected hold has passed, the early-profit check is skipped. Example: `0.25` only triggers in the first 25% of the hold. This captures "fast wins" where the spread decays quickly due to a large favorable move. |
| `--early-profit-min-pct-captured` | `0.60` | Early profit exit: minimum percentage of credit that must be captured. Triggers when at least N% of the original credit has been earned within the early window. Example: `0.60` takes profit when 60% of credit is captured in the first 25% of hold. Higher = harder to trigger, lets more profit accumulate. |
| `--pre-expiration-dte` | `1` | Force-close all positions at DTE <= N, regardless of P&L. This is the hard floor — no position survives past this. Protects against pin risk and auto-assignment. Set to 0 to disable (relies on `--time-decay-exit` instead). |

### Collateral Management

| Flag | Default | Description |
|------|---------|-------------|
| `--max-collateral-pct` | `0.05` | Maximum collateral committed per group as a fraction of account equity. At 0.05, each group (signal) can use up to 5% of the account. |
| `--max-total-collateral-pct` | `0.30` | Maximum total collateral across all open positions. At 0.30, the strategy never commits more than 30% of equity. For single-underlying strategies, consider reducing to 0.20. |

### PDF Retraining

| Flag | Default | Description |
|------|---------|-------------|
| `--retrain-interval` | `None` | Rebuild the PDF during simulation: `weekly` or `monthly`. Uses expanding or rolling window of historical data. |
| `--training-start` | `None` | Start of training data window (YYYY-MM-DD). Defaults to 1 year before `--start`. |
| `--rolling-window` | `None` | Use a rolling window of N days instead of expanding from training-start. |

## Example Configurations

### Conservative (tighter entry filters)
```bash
python demo_credit_spread.py \
  --min-p-profit 0.75 \
  --min-credit-width-ratio 0.25 \
  --max-open-positions 6 \
  --max-positions-per-expiration 2 \
  --max-directional-imbalance 2 \
  --daily-loss-limit 0.03 \
  --max-loss-mult 1.5 \
  --max-collateral-pct 0.03 \
  --max-total-collateral-pct 0.15 \
  --target-dte 21
```

### Short-term reversion mode (original behavior)
```bash
python demo_credit_spread.py \
  --enable-reversion-exit \
  --enable-strike-breach-exit \
  --gamma-risk-dte 7 \
  --pre-expiration-dte 0 \
  --min-otm-pct 0 \
  --cooldown-candles 0 \
  --profit-target 0.50 \
  --max-loss-mult 2.0 \
  --time-decay-exit 5 \
  --target-dte 30
```

### Parameter sweep (for optimization)
```bash
for p in 0.60 0.65 0.70 0.75 0.80; do
  for m in 1.0 1.5 2.0; do
    python demo_credit_spread.py \
      --min-p-profit $p \
      --max-loss-mult $m \
      --symbol AAPL --start 2025-06-01 --end 2025-09-15 \
      2>&1 | tail -5
  done
done
```

## Performance

### Profiling

Add `--profile` to any run to get a timing breakdown:

```bash
python demo_credit_spread.py \
  --symbol AAPL --start 2025-06-01 --end 2025-09-15 \
  --retrain-interval weekly --profile
```

Outputs a table showing count, total time, mean, p50, p95, and % of wall clock per call type (`tick`, `fetch_ladder`, `place_order`, `process_candles`, `on_tick_callback`).

### Ladder Cache

The options ladder is the most expensive operation (~20-35 Polygon API calls per fetch). Two cache layers reduce redundant fetches:

| Layer | Param | Default | Description |
|-------|-------|---------|-------------|
| **Python-side** | `--ladder-cache-minutes` | `15` | Reuses the same ladder across ticks within this window. Option prices don't change meaningfully in 5-15 minutes for strike selection. |
| **Go server-side** | `POLYGON_CACHE_DIR` | `$TRADING_PROJECT_DIR/.cache/polygon/` | Persists aggregate bar data to disk. Survives server restarts — second simulation run on the same date range is dramatically faster. |

### Server Cache

The Go server caches Polygon API responses in memory and persists the aggregate bars bucket to disk:

- **Location:** `$TRADING_PROJECT_DIR/.cache/polygon/aggregate_bars.json` (gitignored)
- **Auto-flush:** Every 50 new cache entries
- **Startup load:** Reads from disk on server start if file exists
- **First run:** ~3-5s per ladder fetch (Polygon API calls)
- **Cached runs:** <100ms per ladder fetch (disk-cached data)

To clear the cache, delete the `.cache/polygon/` directory and restart the server.

## Architecture

```
demo_credit_spread.py          # CLI entry point, argparse, playground setup
credit_spread_strategy.py      # Strategy logic
  CreditSpreadStrategy         #   Main class
    _check_entries()            #   Signal -> entry flow
    _place_entry()              #   Strike selection, risk gates, order placement
    _check_exits()              #   Exit priority chain (8 conditions)
    _place_exit()               #   Close both legs
  run_credit_spread_strategy()  #   Main loop wrapper
mean_reversion_report.py       # Post-run report generation
pdf_types.py                   # PDFDocument, HorizonStats dataclasses
deviation_levels.py            # Sigma-based level computation
```

## EV Model

The expected value (EV) model estimates profit for each spread at entry time. It **adapts automatically to CLI args** so the prediction reflects the configured exit strategy.

### How it works

For each forward-return scenario in the PDF, the model computes the spread's expiration payoff, then applies exit-aware adjustments:

| Adjustment | Controlled by | Effect |
|-----------|---------------|--------|
| **Profit cap** | `--profit-target` | Wins capped at N% of credit. At 0.50, no scenario can contribute more than 50% of the net credit received. |
| **Loss cap** | `--max-loss-mult` | Losses capped at Nx credit. At 2.0, worst-case loss per scenario is 2x the credit. At 0 (default), full expiration loss is used (conservative). |
| **Exit slippage** | Always on | Subtracts estimated bid-ask cost of closing from every scenario. Uses entry-time spreads as a proxy. |

This makes EV a **conservative lower bound** — wins are capped at what the strategy actually targets, while losses default to worst-case expiration payoff unless a stop-loss is configured.

### Modeled vs unmodeled parameters

Parameters **modeled in EV** (directly adjust the payoff):
- `--profit-target` — caps the gain side
- `--max-loss-mult` — caps the loss side

Parameters **not modeled** (logged as warnings at startup):
- `--time-decay-exit` — positions exit early, but EV uses full-expiration losses (conservative)
- `--pre-expiration-dte` — similar; EV overstates losses for positions that exit before expiration
- `--enable-strike-breach-exit`, `--gamma-risk-dte` — forced exits not modeled
- `--early-profit-time-pct` — bounded by the profit_target cap

### Startup diagnostics

The strategy logs its EV model assumptions at startup:

```
------------------------------------------------------------
EV MODEL (adaptive to CLI args)
------------------------------------------------------------
  [EV] Profit capped at 50% of credit (--profit-target 0.5)
  [EV] Losses capped at 2.0x credit (--max-loss-mult 2.0)
  [EV] Exit slippage estimated from bid-ask spreads
  [EV] WARNING: time_decay_exit_dte=5 — early exit not modeled, EV losses may overestimate
  [EV] WARNING: pre_expiration_dte=1 — positions exit before expiration
------------------------------------------------------------
```
