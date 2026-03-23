# PDF-Guided Wheel Strategy

Options wheel strategy that uses empirical probability distributions (PDFs) to guide put strike selection and Kelly-based position sizing. Supports optional periodic PDF retraining during the simulation.

## Architecture

```
Historical candles (LTF 15-min + HTF daily)
        |
        v
  PDFBuilder → PDFDocument (signal distributions)
        |
        v
  Simulation tick loop
    |-- Daily candle: detect signals (SuperTrend, StochRSI, patterns)
    |-- Build compound signal key, look up in PDF
    |-- If match: Kelly sizing → fetch options ladder → sell puts
    |-- If assigned: sell covered calls
    |-- [Optional] Friday close → retrain PDF from updated data
```

## Prerequisites

1. **Go trading server** running on `localhost:5051` (Twirp RPC):
   ```bash
   task app:dev
   ```

2. **Python environment** (`grodt` conda env):
   ```bash
   conda activate grodt
   ```

## Step 1: Build the PDF

```bash
cd src/clients/python

python build_pdf_from_polygon.py \
    --symbol AAPL \
    --start 2024-01-01 \
    --end 2025-01-01 \
    --output aapl_pdf.json
```

See `build_pdf_from_polygon.py --help` for all options (strategy presets, custom timeframes, Bayesian model).

## Step 2: Run the Backtest

### Static PDF (original behavior)

Uses a pre-built PDF that stays fixed throughout the simulation:

```bash
python demo_pdf_wheel_strategy.py \
    --symbol AAPL \
    --start 2025-01-01 \
    --end 2025-06-30 \
    --balance 100000 \
    --pdf-path aapl_pdf.json
```

### With periodic retraining

The PDF is rebuilt during the simulation at each interval boundary (Friday close for weekly, month-end for monthly). No `--pdf-path` needed — the PDF is built dynamically from historical data.

```bash
# Weekly retrain, expanding window (default: 1 year before --start)
python demo_pdf_wheel_strategy.py \
    --symbol AAPL \
    --start 2025-01-01 \
    --end 2025-06-30 \
    --retrain-interval weekly

# Weekly retrain with explicit training start
python demo_pdf_wheel_strategy.py \
    --symbol AAPL \
    --start 2025-01-01 \
    --end 2025-06-30 \
    --retrain-interval weekly \
    --training-start 2023-06-01

# Monthly retrain with rolling 6-month window
python demo_pdf_wheel_strategy.py \
    --symbol AAPL \
    --start 2025-01-01 \
    --end 2025-06-30 \
    --retrain-interval monthly \
    --rolling-window 180

# Bayesian model for retraining
python demo_pdf_wheel_strategy.py \
    --symbol AAPL \
    --start 2025-01-01 \
    --end 2025-06-30 \
    --retrain-interval weekly \
    --return-model bayesian_nig
```

### How retraining works

```
training_start              sim_start                          sim_end
    |---------------------------|=================================|
    |   base training window    |       simulation window         |
    |                           |                                 |
PDF v0 (initial):              |<============================>|  |
PDF v1 (Fri wk1):             |<==============================>| |
PDF v2 (Fri wk2):             |<================================>|
```

**Expanding window** (default): each retrain uses all data from `--training-start` to the current simulation date. More data over time.

**Rolling window** (`--rolling-window N`): each retrain uses only the last N days of data. Drops stale patterns.

At each retrain boundary, the strategy logs:
```
============================================================
PDF RETRAINED #3 @ 2025-01-17 (expanding from 2024-01-01)
  LTF bars: 12,480  |  Daily bars: 252
  Signals: 45 -> 48  |  Sufficient: 12
============================================================
```

### `demo_pdf_wheel_strategy.py` parameters

| Flag | Default | Description |
|------|---------|-------------|
| `--symbol` | `AAPL` | Stock symbol |
| `--start` | `2025-01-01` | Simulation start date |
| `--end` | `2025-06-30` | Simulation end date |
| `--balance` | `100000` | Starting account balance |
| `--pdf-path` | *(none)* | Pre-built PDF JSON (required when not using retrain) |
| `--kelly-fraction` | `0.5` | Kelly fraction multiplier (0.5 = half-Kelly) |
| `--retrain-interval` | *(none)* | `weekly` or `monthly` — enables dynamic PDF retraining |
| `--training-start` | *(1 year before --start)* | Start of training data window |
| `--rolling-window` | *(none)* | Use rolling window of N days instead of expanding |
| `--return-model` | `empirical` | `empirical` or `bayesian_nig` |
| `--twirp-host` | `http://127.0.0.1:5051` | Go server address |

## CLI Examples

### End-to-end: build PDF then run static backtest

```bash
cd src/clients/python

# 1. Build PDF from 1 year of historical data
python build_pdf_from_polygon.py \
    --symbol AAPL \
    --start 2024-01-01 \
    --end 2025-01-01 \
    --output aapl_pdf.json

# 2. Run backtest with that PDF
python demo_pdf_wheel_strategy.py \
    --symbol AAPL \
    --start 2025-01-01 \
    --end 2025-06-30 \
    --balance 100000 \
    --pdf-path aapl_pdf.json \
    --kelly-fraction 0.5
```

### Weekly retrain with defaults

No need to pre-build a PDF — training data is fetched automatically. Defaults to an expanding window starting 1 year before `--start`:

```bash
python demo_pdf_wheel_strategy.py \
    --symbol AAPL \
    --start 2025-01-01 \
    --end 2025-06-30 \
    --balance 100000 \
    --retrain-interval weekly
```

### Weekly retrain with 2-year training history

```bash
python demo_pdf_wheel_strategy.py \
    --symbol AAPL \
    --start 2025-01-01 \
    --end 2025-06-30 \
    --balance 100000 \
    --retrain-interval weekly \
    --training-start 2023-01-01
```

### Monthly retrain with rolling 6-month window

Older data is dropped — only the most recent 180 days are used at each retrain:

```bash
python demo_pdf_wheel_strategy.py \
    --symbol AAPL \
    --start 2025-01-01 \
    --end 2025-06-30 \
    --balance 100000 \
    --retrain-interval monthly \
    --rolling-window 180
```

### Bayesian model with aggressive Kelly

```bash
python demo_pdf_wheel_strategy.py \
    --symbol AAPL \
    --start 2025-01-01 \
    --end 2025-06-30 \
    --balance 100000 \
    --retrain-interval weekly \
    --return-model bayesian_nig \
    --kelly-fraction 0.75
```

### Compare static vs retrained

```bash
# Static (baseline)
python demo_pdf_wheel_strategy.py \
    --symbol AAPL \
    --start 2025-01-01 \
    --end 2025-06-30 \
    --pdf-path aapl_pdf.json

# Retrained (same sim window, weekly updates)
python demo_pdf_wheel_strategy.py \
    --symbol AAPL \
    --start 2025-01-01 \
    --end 2025-06-30 \
    --retrain-interval weekly
```

## Tests

```bash
cd src/clients/python

# PDF builder tests
/Users/jamal/miniconda3/envs/grodt/bin/python -m pytest test_pdf_builder.py -v

# Wheel strategy tests
/Users/jamal/miniconda3/envs/grodt/bin/python -m pytest test_demo_covered_call.py -v
```
