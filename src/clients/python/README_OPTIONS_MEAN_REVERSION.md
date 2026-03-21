# Options Mean-Reversion Strategy

PDF-guided options strategy that buys calls on bullish dips and puts on bearish rallies, using the same multi-timeframe signal detection as the stock mean-reversion strategy.

## How It Works

1. **HTF (1-hour)**: Detects compound signals via PDF (e.g., `bullish_supertrend|stochrsi_cross_above_20`)
2. **Signal direction**: Positive mean forward return → bullish (buy calls). Negative → bearish (buy puts).
3. **LTF (5-min)**: When price dips (bullish) or rallies (bearish) to a σ-deviation band, buys options at the corresponding strike
4. **Exits**: Profit target (primary), reversion complete, or time decay threshold
5. **Risk**: Capped at premium paid — no stop-out logic needed

## Key Differences from Stock Strategy

| Feature | Stock (`mean_reversion_strategy.py`) | Options (`options_mean_reversion_strategy.py`) |
|---------|--------------------------------------|------------------------------------------------|
| Direction | Long-only (bullish) | Both (bullish calls + bearish puts) |
| Entry | Buy shares at deviation levels | BUY_TO_OPEN calls/puts at corresponding strikes |
| Exit | Partial tiers as price recovers | SELL_TO_CLOSE entire position per level |
| Risk limit | Stop-out at 95th percentile HTF move | Premium paid = max loss |
| Position sizing | Shares proportional to P(revert) | Contracts proportional to P(revert) |
| Budget | `max_loss_pct × equity` (loss-based) | `max_premium_pct × equity` (premium-based) |
| Expected profit | Uses unbounded `p_revert` (appropriate for no-expiry stock) | Uses **time-bounded** `p_revert` from longest PDF horizon |

## Files

| File | Purpose |
|------|---------|
| `options_mean_reversion_strategy.py` | Strategy class + `run_options_mean_reversion()` runner |
| `demo_options_mean_reversion.py` | CLI demo script with PDF loading/retraining |
| `test_options_mean_reversion_strategy.py` | 48 unit tests |

## Usage

```bash
# Static PDF
python demo_options_mean_reversion.py \
    --symbol AAPL --start 2025-06-01 --end 2026-02-28 \
    --balance 100000 --pdf-path aapl_5m_1h_pdf.json

# Weekly retraining with Bayesian model
python demo_options_mean_reversion.py \
    --symbol AAPL --start 2025-06-01 --end 2026-02-28 \
    --balance 100000 --retrain-interval weekly --model bayesian_nig \
    --rolling-window 180

# ITM calls only, higher profit target
python demo_options_mean_reversion.py \
    --symbol AAPL --start 2025-06-01 --end 2026-02-28 \
    --balance 100000 --pdf-path aapl_5m_1h_pdf.json \
    --strike-strategy model --profit-target 1.0 --target-dte 21
```

## CLI Arguments

### Strategy parameters
| Flag | Default | Description |
|------|---------|-------------|
| `--max-premium-pct` | `0.02` | Max premium per group as % of equity |
| `--total-contracts` | `10` | Contracts to distribute across deviation levels |
| `--strike-strategy` | `model` | Strike selection: `model`, `atm`, or `otm` |
| `--profit-target` | `0.50` | Exit when option value is this fraction above premium |
| `--target-dte` | `14` | Preferred days to expiration |
| `--min-dte` / `--max-dte` | `5` / `30` | Acceptable DTE range |
| `--time-decay-exit` | `3` | Force exit when DTE drops below this |
| `--stop-percentile` | `0.95` | Used for deviation plan computation |
| `--min-hold-candles` | `6` | Min LTF candles to hold before exit (6 = 30 min) |
| `--tail-threshold` | `0.005` | Tail threshold for auto sigma steps (lower = more levels) |
| `--min-expected-profit` | `50.0` | Skip entries with expected profit below this ($) |
| `--long-only` | `false` | Only trade bullish signals (calls on dips), skip bearish/puts |

### PDF / retraining
| Flag | Default | Description |
|------|---------|-------------|
| `--pdf-path` | | Path to pre-built PDF JSON |
| `--retrain-interval` | | `weekly` or `monthly` PDF rebuild |
| `--model` | `bayesian_nig` | Return model (`empirical` or `bayesian_nig`) |
| `--rolling-window` | | Rolling window in days (vs expanding) |
| `--training-start` | | Training data start date |

## Strike Selection Strategies

### `model` (default)
Always selects **ITM** strikes to guarantee intrinsic value at signal price. Depth increases with sigma distance:
- **Shallow dip** (σ < 1.5): Moderately ITM — strike offset 0.5× below level price
- **Medium dip** (1.5 ≤ σ < 2.5): Deeper ITM — strike offset 1.0× below level price
- **Deep dip** (σ ≥ 2.5): Very deep ITM — strike offset 1.5× below level price

For bearish direction, ITM means strikes are offset above the level price.

### `atm`
Always picks the strike closest to the current stock price at entry time.

### `otm`
Always picks OTM strikes — above stock price for calls, below for puts. Cheapest premium, maximum leverage.

## Exit Conditions

Checked on every LTF candle (after `min_hold_candles`), in priority order:

1. **Profit target** (primary): Option position value ≥ `(1 + profit_target_pct) × premium_paid` → close that entry
2. **Reversion complete**: Stock price reaches the HTF signal price → close ALL entries in the group
3. **Time decay**: DTE ≤ `time_decay_exit_dte` → force close to avoid theta acceleration
4. **End of simulation**: All remaining positions liquidated

## Entry Flow

```
HTF candle closes
  → detect_atomic_signals_on_bar()
  → compound signal key lookup in PDF
  → if mean > 0: bullish group (calls on dips)
  → if mean < 0: bearish group (puts on rallies)
  → compute_deviation_levels() for entry price targets

LTF candle closes
  → bullish: candle_low ≤ level.price → triggered
  → bearish: candle_high ≥ level.price → triggered
  → fetch_ladder() for available option contracts
  → _select_contract() picks best strike per strike_strategy
  → budget check: contracts × ask × 100 ≤ remaining budget
  → compute expected profit (with time-bounded p_revert)
  → skip if expected_profit < min_expected_profit
  → place_order(BUY_TO_OPEN, "option")
```

## Expected Profit Formula

Uses **time-bounded reversion probability** — the fraction of forward returns at the longest available PDF horizon (e.g., 2w) that ended at or above signal price — instead of the unbounded `p_revert` from deviation levels. This prevents overestimating reversion likelihood for time-limited options positions.

For calls (bullish), `p_revert_bounded = P(longest_horizon_return >= 0)`:
```
intrinsic_at_signal = max(0, signal_price - strike) × 100 × contracts
expected_profit = p_revert_bounded × intrinsic_at_signal - total_premium
```

For puts (bearish), `p_revert_bounded = P(longest_horizon_return <= 0)`:
```
intrinsic_at_signal = max(0, strike - signal_price) × 100 × contracts
expected_profit = p_revert_bounded × intrinsic_at_signal - total_premium
```

## PDF Generation

The PDF builder computes forward returns at multiple horizons from historical LTF bar data. The options demo uses 5-min bars with these horizons:

| Horizon | 5-min bars | Calendar time |
|---------|-----------|---------------|
| `1h`    | 12        | 1 hour |
| `4h`    | 48        | 4 hours |
| `1d`    | 78        | 1 trading day (6.5 hours) |
| `1w`    | 390       | 1 trading week (5 days) |
| `2w`    | 780       | 2 trading weeks (10 days) |

Longer horizons (`1w`, `2w`) are critical for options because the time-bounded p_revert uses the longest available horizon to estimate whether reversion happens within the option's lifetime (typically 14 DTE). Without these, the estimate relies on shorter windows that overstate reversion probability.

**Note**: Longer horizons require more historical data (signals near the end of the dataset won't have enough lookahead bars) and may reduce sample sizes. This is an acceptable trade-off for more accurate expected profit estimates.

### Building a PDF

The PDF is built by `PDFBuilder` which fetches historical candle data, detects compound signals, and computes forward returns at each horizon. There are two ways to build it:

**Option A: Build inline during the backtest** (recommended for iteration)

The demo script rebuilds the PDF at a configurable interval during simulation:

```bash
# Build once at the start (default — no retrain flag)
python demo_options_mean_reversion.py \
    --symbol AAPL --start 2025-06-01 --end 2026-02-28 \
    --balance 100000 --model bayesian_nig

# Rebuild weekly using a 180-day rolling window
python demo_options_mean_reversion.py \
    --symbol AAPL --start 2025-06-01 --end 2026-02-28 \
    --balance 100000 --retrain-interval weekly --model bayesian_nig \
    --rolling-window 180

# Rebuild monthly with expanding window from a fixed start
python demo_options_mean_reversion.py \
    --symbol AAPL --start 2025-06-01 --end 2026-02-28 \
    --balance 100000 --retrain-interval monthly --model empirical \
    --training-start 2024-06-01
```

When no `--pdf-path` is provided, the demo creates a temporary playground to fetch candle data from Polygon, builds the PDF using `PDFBuilder`, then creates the real playground for simulation.

**Option B: Pre-build and save to JSON** (for reuse across runs)

Build the PDF once in Python and save it, then reference it in future runs:

```python
from pdf_builder import PDFBuilder
from backtester_playground_client_grpc import BacktesterPlaygroundClient, ...

# Fetch historical bars (requires running backtester server on :5051)
ltf_bars, htf_bars = fetch_bars(symbol="AAPL", start="2024-06-01", end="2025-06-01")

builder = PDFBuilder(
    symbol="AAPL",
    ltf_bars=ltf_bars,
    daily_bars=htf_bars,
    ltf_period_seconds=300,                                    # 5-min bars
    horizons={"1h": 12, "4h": 48, "1d": 78, "1w": 390, "2w": 780},
    htf_timeframe="ltf",                                       # HTF = 1-hour (via LTF bars)
    return_model="bayesian_nig",                               # or "empirical"
)
pdf = builder.build()
pdf.save("aapl_5m_1h_pdf.json")
```

Then load it:

```bash
python demo_options_mean_reversion.py \
    --symbol AAPL --start 2025-06-01 --end 2026-02-28 \
    --balance 100000 --pdf-path aapl_5m_1h_pdf.json
```

### Inspecting a PDF

```python
from pdf_types import PDFDocument

pdf = PDFDocument.load("aapl_5m_1h_pdf.json")

# List all signals and their sample sizes
for key, signal in pdf.signals.items():
    print(f"{key}: n={signal.sample_size}, sufficient={signal.sufficient_samples}")
    for h_name, h_stats in signal.horizons.items():
        print(f"  {h_name}: mean={h_stats.mean:.5f}, std={h_stats.stddev:.5f}, "
              f"n_returns={len(h_stats.forward_returns)}")
```

## Reused Modules

- `deviation_levels.py` — Level placement & contract allocation (shares field = contracts)
- `pdf_types.py` / `pdf_builder.py` — Signal detection and PDF statistics
- `return_models.py` — Bayesian NIG / Empirical models
- `risk_management.py` — `allocate_contracts()` for proportional sizing
- `backtester_playground_client_grpc.py` — `fetch_ladder()`, `place_order()`

## Running Tests

```bash
cd src/clients/python
/Users/jamal/miniconda3/envs/grodt/bin/python -m pytest test_options_mean_reversion_strategy.py -v
```
