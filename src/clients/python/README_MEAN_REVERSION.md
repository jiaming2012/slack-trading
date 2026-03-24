# PDF Mean-Reversion Strategy

Multi-timeframe equity strategy that buys dips at sigma-deviation bands when the higher timeframe signals bullish, using PDF-derived forward-return distributions to size positions and set stops.

## Architecture

```
HTF (1-hour) signal detected
        |
        v
  PDF lookup: is signal bullish? (mean forward return > 0)
        |
        v
  Compute deviation levels from LTF (5-min) PDF
  (sigma bands where P(revert) is highest get more shares)
        |
        v
  LTF candle dips to level -> BUY entry
        |
        v
  Price recovers -> partial exits at tiers toward signal price
        |
  HTF closes beyond 95th percentile adverse -> stop out entire group
```

## Modules

| Module | Purpose |
|--------|---------|
| `deviation_levels.py` | PDF-adaptive level placement + share sizing |
| `partial_exit_manager.py` | Partial exit tier computation and monitoring |
| `return_models.py` | Pluggable statistical models (Empirical, Bayesian NIG) |
| `mean_reversion_strategy.py` | Strategy class, tick loop, order placement |
| `mean_reversion_report.py` | Post-simulation evaluation (expected vs realized P&L) |
| `demo_mean_reversion.py` | CLI script to run a backtest or live trade |
| `build_pdf_from_polygon.py` | CLI script to build PDF JSON files from Polygon data |

## Prerequisites

1. **Go trading server** running on `localhost:5051` (Twirp RPC):
   ```bash
   task app:dev
   ```

2. **Python environment** (`grodt` conda env):
   ```bash
   conda activate grodt
   ```

3. **Dependencies**: numpy, scipy, pandas, loguru, python-dateutil (all included in the grodt env).

## Step 1: Build the PDF

The strategy needs a pre-built PDF JSON file that contains forward-return distributions for compound signals. This file maps signal patterns (e.g., `bullish_supertrend|stochrsi_cross_above_20`) to their historical return distributions at various horizons.

`build_pdf_from_polygon.py` supports two strategy presets and custom timeframes:

### For the wheel strategy (15-min LTF + daily HTF)

```bash
cd src/clients/python

python build_pdf_from_polygon.py \
    --strategy wheel \
    --symbol AAPL \
    --start 2024-06-01 \
    --end 2025-06-01 \
    --output aapl_pdf.json
```

`--strategy wheel` is the default, so `--strategy` can be omitted for backward compatibility.

### For the mean-reversion strategy (5-min LTF + 1-hour HTF)

```bash
python build_pdf_from_polygon.py \
    --strategy mean_reversion \
    --symbol AAPL \
    --start 2024-06-01 \
    --end 2025-06-01 \
    --output aapl_5m_1h_pdf.json
```

### Custom timeframes

Override the preset with `--ltf` and `--htf` flags using shorthand notation (`5m`, `15m`, `1h`, `1d`):

```bash
python build_pdf_from_polygon.py \
    --ltf 5m --htf 1h \
    --symbol AAPL \
    --start 2024-06-01 \
    --end 2025-06-01 \
    --output custom_pdf.json
```

### Using a Bayesian model

Add `--return-model bayesian_nig` for smoother estimates with small sample sizes:

```bash
python build_pdf_from_polygon.py \
    --strategy mean_reversion \
    --return-model bayesian_nig \
    --symbol AAPL \
    --start 2024-06-01 \
    --end 2025-06-01 \
    --output aapl_5m_1h_bayesian_pdf.json
```

### `build_pdf_from_polygon.py` parameters

| Flag | Default | Description |
|------|---------|-------------|
| `--strategy` | `wheel` | Preset: `wheel` (15m/daily) or `mean_reversion` (5m/1h) |
| `--ltf` | *(from preset)* | Custom LTF timeframe, e.g. `5m`, `15m` (overrides preset) |
| `--htf` | *(from preset)* | Custom HTF timeframe, e.g. `1h`, `1d` (overrides preset) |
| `--symbol` | `AAPL` | Stock symbol |
| `--start` | `2024-06-01` | Scan start date |
| `--end` | `2025-06-01` | Scan end date |
| `--return-model` | `empirical` | `empirical` or `bayesian_nig` |
| `--output` | `<symbol>_pdf.json` | Output JSON path |
| `--twirp-host` | `http://127.0.0.1:5051` | Go server address |

## Step 2: Run the Backtest

```bash
cd src/clients/python

python demo_mean_reversion.py \
    --symbol AAPL \
    --start 2025-06-01 \
    --end 2026-02-28 \
    --balance 100000 \
    --pdf-path aapl_5m_1h_pdf.json \
    --max-loss-pct 0.02 \
    --stop-percentile 0.95 \
    --model bayesian_nig
```

### `demo_mean_reversion.py` parameters

| Flag | Default | Description |
|------|---------|-------------|
| `--symbol` | `AAPL` | Stock symbol |
| `--start` | `2025-06-01` | Simulation start date |
| `--end` | `2026-02-28` | Simulation end date |
| `--balance` | `100000` | Starting account balance |
| `--pdf-path` | *(required)* | Path to pre-built PDF JSON file |
| `--max-loss-pct` | `0.02` | Max potential loss per group as fraction of equity |
| `--stop-percentile` | `0.95` | HTF stop percentile (higher = wider stop) |
| `--model` | `bayesian_nig` | Return model: `empirical` or `bayesian_nig` |
| `--total-shares` | `1000` | Total shares to distribute across entry levels per group |
| `--exit-tiers` | `3` | Number of partial exit tiers per entry level |
| `--htf-horizon` | `1h` | Which PDF horizon to use for deviation calculations |
| `--tier-spacing` | `even` | Exit tier spacing: `even` (25%,50%,100%) or `tight` (70%,85%,100%) |
| `--stop-widen-on-exit` | `1.0` | Stop widening factor for partial exits (0.0=disabled) |
| `--min-expected-profit` | `0.0` | Skip entries with expected profit below threshold |
| `--ev-model` | `distribution` | EV model: `binary` or `distribution` |
| `--live` | *(off)* | Run live: `--live` (paper) or `--live margin` (real money) |
| `--retrain-interval` | *(off)* | Rebuild PDF periodically: `weekly` or `monthly` |
| `--training-start` | *(1yr before --start)* | Start of training data window (with retrain) |
| `--rolling-window` | *(expanding)* | Rolling window in days (with retrain) |
| `--twirp-host` | `http://127.0.0.1:5051` | Go server address |

## Step 2b: Run Live (Paper Money)

To run the strategy against the Tradier paper money account in real time:

```bash
cd src/clients/python

python demo_mean_reversion.py \
    --live \
    --symbol AAPL \
    --balance 100000 \
    --pdf-path aapl_5m_1h_pdf.json \
    --max-loss-pct 0.02 \
    --stop-percentile 0.95 \
    --model bayesian_nig
```

- `--live` defaults to the paper (sandbox) account. Use `--live margin` for real money (requires confirmation prompt).
- The strategy runs indefinitely, ticking every ~20 seconds. Press **Ctrl+C** to stop gracefully.
- `--start`/`--end` are not used in live mode (the server streams real-time market data).
- `--retrain-interval` is not supported in live mode — provide a pre-built PDF via `--pdf-path`.

### Use --exit-tiers 1 to make all shares exit at signal_price — no early profit-taking:

python demo_mean_reversion.py \
    --symbol AAPL --start 2025-06-01 --end 2026-02-28 \
    --balance 100000 --retrain-interval weekly \
    --max-loss-pct 0.02 --stop-percentile 0.95 \
    --model bayesian_nig --rolling-window 180 \
    --exit-tiers 1

### Use --tier-spacing tight to cluster exit tiers near signal_price (70%, 85%, 100% of distance) instead of evenly spaced (25%, 50%, 100%):

python demo_mean_reversion.py \
    --symbol AAPL --start 2025-06-01 --end 2026-02-28 \
    --balance 100000 --retrain-interval weekly \
    --max-loss-pct 0.02 --stop-percentile 0.95 \
    --model bayesian_nig --rolling-window 180 \
    --tier-spacing tight

## Step 3: Evaluate Results

The strategy logs a funnel summary at the end showing:
- HTF/LTF bars processed
- Signals detected, groups created/skipped/truncated
- Entries placed, exits placed, stop-outs
- Group status breakdown (active, closed, stopped out)

For detailed post-simulation analysis, use the report module. It fetches all order data directly from the server using order attributes (tags) set at placement time — no in-memory strategy state needed:

```bash
# CLI usage (standalone — only needs playground ID)
python mean_reversion_report.py \
    --playground-id <UUID> \
    --twirp-host http://localhost:5051 \
    --output mean_rev_report.csv
```

```python
# Python usage
from mean_reversion_report import generate_mean_reversion_report

order_df, group_df, metrics = generate_mean_reversion_report(
    playground_id="<UUID>",
    twirp_host="http://localhost:5051",
    output_path="mean_rev_report.csv",
)

print(f"MAE: ${metrics.mae:.2f}")
print(f"Directional accuracy: {metrics.directional_accuracy:.1%}")
print(f"Total expected: ${metrics.total_expected:.2f}")
print(f"Total realized: ${metrics.total_realized:.2f}")
```

### `mean_reversion_report.py` parameters

| Flag | Default | Description |
|------|---------|-------------|
| `--playground-id` | *(required)* | UUID of the completed playground |
| `--twirp-host` | `http://localhost:5051` | Go server address |
| `--output` | *(none)* | CSV output path (optional) |

### Report columns

The order-level CSV reconstructs everything from order attributes:

| Column | Source |
|--------|--------|
| `order_id` | Server order ID |
| `group_id` | Order attribute |
| `htf_signal` | Order attribute |
| `signal_price` | Order attribute |
| `action` | Order attribute (`entry`, `partial_exit`, `stop_out`, `end_of_sim_close`) |
| `symbol`, `side`, `quantity` | Order fields |
| `fill_price` | First trade fill price |
| `sigma_distance`, `p_revert` | Order attributes (entries only) |
| `expected_profit` | Order attribute (model prediction at entry time) |
| `model_name` | Order attribute (`empirical` or `bayesian_nig`) |
| `stop_price` | Order attribute |
| `pl` | Server-computed realized P&L |

The group-level summary aggregates by `group_id` and computes prediction error, directional accuracy, and group status (inferred from order types present).

## How It Works

### Entry Logic

1. HTF candle closes -> detect atomic signals (SuperTrend, StochRSI, candlestick patterns)
2. Build compound signal key, look up in PDF
3. If bullish (mean forward return > 0, sufficient samples):
   - Compute sigma-deviation entry levels from the PDF's standard deviation
   - Allocate more shares to deeper levels (where P(revert) is higher)
   - Cap total potential loss at `max_loss_pct` of equity, truncating deepest levels if needed
4. On subsequent LTF candles, if `candle.low <= level.price`, place BUY order

### Exit Logic

- **Partial exits**: As price recovers, sell shares at evenly-spaced tiers between entry and signal price
- **Stop-out**: If HTF bar closes beyond the 95th percentile adverse move, sell all remaining shares in the group
- **End-of-sim**: Any remaining active positions are closed at market

### Statistical Models

| Model | Best For | How It Works |
|-------|----------|--------------|
| `empirical` | Large samples (>100) | Raw histogram percentiles, frequentist CI |
| `bayesian_nig` | Small samples (10-50) | Normal-InverseGamma conjugate prior, Student-t predictive distribution, shrinks toward pooled prior when data is sparse |

## Tests

```bash
cd src/clients/python

# All mean-reversion tests (128 tests)
/Users/jamal/miniconda3/envs/grodt/bin/python -m pytest \
    test_deviation_levels.py \
    test_partial_exit_manager.py \
    test_return_models.py \
    test_mean_reversion_strategy.py \
    test_mean_reversion_report.py \
    -v

# Existing PDF builder tests (44 tests)
/Users/jamal/miniconda3/envs/grodt/bin/python -m pytest test_pdf_builder.py -v
```
