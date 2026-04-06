# Trading Stack — Full Architecture & Plan

## System Purpose

A five-component feedback loop that surfaces stock candidates, evaluates them deterministically, tunes execution and scanning behavior, validates simulator fidelity against live trades, and tracks real-money EV over time. The system gets smarter each cycle without manual intervention.

---

## Component Overview

| Component | Type | Role |
|---|---|---|
| AI Scanner | AI | Surface and rank candidates |
| Simulator | Deterministic | Ground truth trade evaluation |
| Strategy Optimizer | AI | Tune how trades are executed |
| Scanner Optimizer | AI | Tune what gets scanned |
| Fidelity Checker | Deterministic | Validate simulator vs live trades |
| EV Tracker | Analytical | Measure real-money strategy health over time |

---

## Full Architecture

```
DATA LAYER
(Market Feed · Fundamentals · Alt Data · Macro Tags)
    │
    ▼
AI SCANNER ◄─────────────────────────────── SCANNER OPTIMIZER ◄──────────────┐
    │                                               ▲                         │
    │  candidates + feature vectors                 │ ev_weights              │
    ▼                                               │                         │
SIMULATOR ──────────────────────────────────────────┤               EV TRACKER
    │                                               │                         ▲
    ├──► sim trade records ──► STRATEGY OPTIMIZER   │                         │
    │         (weekly)              │               │                         │
    │                               └──► updated strategy config              │
    │                                         │                               │
    ▼                                         ▼                               │
LIVE TRADES ────────────────────────────► FIDELITY CHECKER                   │
    │                                          │                              │
    │                                   drift coefficient                     │
    │                                          │                              │
    └──────────────────────────────────────────┴─────────────────────────────┘
                                    live trade results
```

---

## Feedback Loops

Three loops running at different frequencies:

```
Fast loop   (weekly):   Simulator → Strategy Optimizer → Simulator
Medium loop (monthly):  Simulator → Scanner Optimizer → Scanner
Slow loop   (quarterly):EV Tracker → Scanner Optimizer weights → everything upstream
```

The EV tracker loop is the slowest but highest-trust — it is the only component touching real-money outcomes.

---

## Components

---

### 1. Data Layer

- Real-time market feed (price, volume, OHLC)
- Fundamental ratios (P/E, revenue growth, short interest)
- Alternative data (options flow, macro tags)
- Point-in-time universe — includes delisted tickers to avoid survivorship bias

---

### 2. AI Scanner

**Role:** Surface and rank candidate stocks worth simulating. Never runs simulations itself.

#### Three Layers

**Layer 1 — Hard Filters (Rule-based)**
Fast elimination pass. Runs first, cheapest to compute.

```
Liquidity Gate:       avg daily volume > 500k, price > $5, market cap > $300M
Data Quality Gate:    no earnings within 3 days, no recent halts, options chain exists
Regime Filter:        ATR% and MA filters conditional on current regime tag
```

**Layer 2 — Feature Extraction**

| Feature | Description |
|---|---|
| `rsi_14` | Momentum signal |
| `volume_ratio` | Today's volume / 20d avg (pace-adjusted intraday) |
| `atr_pct` | ATR / price — normalized volatility |
| `price_vs_50ma` | % above or below 50-day moving average |
| `compression_score` | Bollinger Band width percentile |
| `short_interest` | Contrarian / squeeze signal |
| `sector_momentum` | Sector ETF 10d return |
| `regime_tag` | trending / mean_reverting / high_vol |

**Layer 3 — ML Ranking (XGBoost)**

- Scores each candidate 0–1
- Score = probability this candidate produces a winning simulation outcome
- Trained on `scan_results JOIN sim_outcomes` labeled data
- Regime-conditional: separate model weights per regime tag

**Layer 4 — Output**
- Top N candidates (configurable) pushed to simulator queue
- Full feature vector written to `scan_results` at scan time
- `scanner_version` tagged on every record

#### Feature Vector Accuracy Rules

- Every feature stamped with `data_as_of` timestamp
- `data_as_of` must be ≤ `scanned_at` — violations flagged before optimizer training
- Raw feature values stored at scan time — never recomputed retroactively
- Volume ratio pace-adjusted intraday to avoid time-of-day bias
- Point-in-time universe to prevent survivorship bias

---

### 3. Simulator

**Role:** Deterministic ground truth engine. No AI. Never modified to match live results.

- Runs configurable strategy rules against each candidate
- Supports multiple strategies per scan batch (3x labeled data, 1x scan cost)
- Tracks: PnL, drawdown, Sharpe, win rate, hold time, exit reason
- Exit reasons: `stop` | `target` | `timeout` | `signal_exit`
- Writes results to `sim_outcomes` keyed to `scan_result_id`

> If most losses are `stop` hits within 1–2 days → entry timing is off, not the thesis.
> If losses are `timeout` → thesis may be right but catalyst isn't materializing fast enough.

---

### 4. Strategy Optimizer

**Role:** Tune how trades are executed.

- **Input:** `sim_outcomes` trade records
- **Tunes:** stop %, target %, hold days, position sizing, entry trigger rules
- **Method:** Bayesian optimization over strategy parameter space
- **Output:** Updated strategy config → Simulator
- **Frequency:** Weekly or on regime change

---

### 5. Scanner Optimizer

**Role:** Tune what gets scanned.

- **Input:** `scan_results JOIN sim_outcomes` as labeled training data, weighted by `strategy_ev_weights`
- **Tunes:** Feature weights per regime, hard filter thresholds, score cutoffs
- **Method:** XGBoost retraining + SHAP feature importance analysis
- **Output:** Updated scanner config → Scanner
- **Frequency:** Monthly (structural signal, not noise)

#### How EV Tracker Feeds In

The scanner optimizer adjusts training row weights based on live EV signal:

```sql
SELECT
    sr.*,
    so.outcome_label,
    ev.ev_weight        -- 0.0 to 1.0, derived from EV tracker
FROM scan_results sr
JOIN sim_outcomes so ON so.scan_result_id = sr.id
JOIN strategy_ev_weights ev
    ON ev.strategy_id = so.strategy_id
    AND ev.regime     = sr.regime_tag
WHERE sr.scanned_at < cutoff_date
```

Weight logic:

```
ev_slope > 0.1   → ev_weight = 1.0   (full trust, strategy improving)
ev_slope 0–0.1   → ev_weight = 0.7   (moderate trust, stable)
ev_slope < 0     → ev_weight = 0.3   (downweight, strategy decaying)
ev_weight = 0    → strategy retired, rows excluded entirely
```

Without EV feedback the scanner optimizer would keep reinforcing features that historically worked for a decaying strategy — the model would look good on sim data while live performance quietly degrades.

#### Scanner Config Payload (Optimizer Output)

```json
{
  "version": "2026-04-03T06:00:00Z",
  "regime_models": {
    "trending": {
      "feature_weights": {
        "volume_ratio": 0.31,
        "price_vs_50ma": 0.28,
        "rsi_14": 0.12,
        "short_interest": 0.09,
        "compression_score": 0.08,
        "sector_momentum": 0.12
      },
      "drop_features": ["pe_ratio"],
      "hard_filter_overrides": {
        "volume_ratio_floor": 1.2,
        "atr_pct_ceiling": 3.5
      },
      "score_threshold": 0.65
    },
    "high_vol": {
      "hard_filter_overrides": {
        "volume_ratio_floor": 1.8,
        "atr_pct_ceiling": 2.8
      },
      "score_threshold": 0.72
    }
  },
  "global": {
    "top_n_candidates": 20,
    "min_labeled_samples": 500
  }
}
```

Scanner polls config version at the start of each cycle and hot-swaps if changed — no restart required. All config versions stored for rollback by ID.

---

### 6. Fidelity Checker

**Role:** Validate that the simulator accurately mirrors live trading reality.

**Why it matters:** The optimizers are only as trustworthy as the simulator. If simulator drift is high, optimizer output is garbage.

#### What It Detects

| Drift Source | Description |
|---|---|
| Fill slippage | Simulator assumes clean fills, live gets partial fills |
| Spread costs | Simulator ignores bid/ask, live eats it on entry/exit |
| Timing lag | Simulator enters at signal bar close, live is 1–2 seconds late |
| Liquidity limits | Simulator assumes full size filled, live gets partial |
| Borrow costs | Short positions have costs simulator may not model |

#### How It Works

```
Live trades (week)
        │
        ▼
Re-run simulator over same time period
        │
        ▼
Compare: PnL, fill prices, hold time, exit reason
        │
        ▼
Compute drift score per strategy
        │
        ├── drift within tolerance → optimizers proceed normally
        └── drift exceeds threshold → optimizers pause, alert raised
```

#### Output

A **drift coefficient per strategy** stored in `simulator_fidelity`. Optimizers weight their training data accordingly — high drift periods are downweighted or excluded.

> Build this before the optimizers. It is a prerequisite for trusting the rest of the system.

---

### 7. EV Tracker

**Role:** Measure whether strategies are generating positive expected value in live trading, and detect decay over time.

#### EV Formula

```
EV = (win_rate × avg_win) − (loss_rate × avg_loss)
```

Win rate alone is misleading. A 40% win rate can be highly profitable if winners are 3x the size of losers. EV captures both dimensions.

#### What It Tracks

- EV per strategy per regime (rolling 30d / 90d / all-time)
- EV trend slope — is this strategy's edge growing or decaying?
- Strategy ranking by EV
- Regime decay detection — edge being arbitraged away or regime has shifted

#### EV Decay Detection

```
slope > 0   → EV improving, scale up
slope ≈ 0   → stable, maintain
slope < 0   → EV decaying, flag for review or retirement
```

A decaying slope caught before the optimizers detect it provides an early warning that something structural has changed — new market participants, regime shift, or edge erosion.

#### Future: Recommendation Engine

Once 6–12 months of EV history exists per strategy per regime, the recommendation layer is mostly pattern matching on that data:

- Retire Strategy X (EV decaying 3 consecutive regimes)
- Scale Strategy Y (EV improving, fidelity high)
- Pause scanning for Strategy Z in high_vol regime (historically negative EV)

The hard work is the tracking infrastructure. The recommendations follow naturally.

---

## Database Schema

### `scan_results`

```sql
CREATE TABLE scan_results (
    id              UUID PRIMARY KEY,
    scanned_at      TIMESTAMPTZ NOT NULL,
    ticker          TEXT NOT NULL,
    regime_tag      TEXT,
    regime_confidence NUMERIC,
    price           NUMERIC,
    volume_ratio    NUMERIC,
    rsi_14          NUMERIC,
    atr_pct         NUMERIC,
    short_interest  NUMERIC,
    sector          TEXT,
    scanner_score   NUMERIC,
    scanner_version TEXT,
    data_as_of      TIMESTAMPTZ   -- must be <= scanned_at
);
```

### `sim_outcomes`

```sql
CREATE TABLE sim_outcomes (
    id              UUID PRIMARY KEY,
    scan_result_id  UUID REFERENCES scan_results(id),
    simulated_at    TIMESTAMPTZ NOT NULL,
    strategy_id     TEXT,
    entry_price     NUMERIC,
    exit_price      NUMERIC,
    stop_price      NUMERIC,
    target_price    NUMERIC,
    pnl_pct         NUMERIC,
    hold_days       INTEGER,
    exit_reason     TEXT,
    max_drawdown    NUMERIC,
    outcome_label   TEXT
);
```

### `simulator_fidelity`

```sql
CREATE TABLE simulator_fidelity (
    id              UUID PRIMARY KEY,
    computed_at     TIMESTAMPTZ,
    strategy_id     TEXT,
    period_start    TIMESTAMPTZ,
    period_end      TIMESTAMPTZ,
    drift_pnl       NUMERIC,     -- sim PnL minus live PnL
    drift_fill      NUMERIC,     -- avg fill price delta
    drift_score     NUMERIC,     -- composite 0.0 to 1.0
    within_tolerance BOOLEAN
);
```

### `strategy_ev_weights`

```sql
CREATE TABLE strategy_ev_weights (
    id              UUID PRIMARY KEY,
    computed_at     TIMESTAMPTZ,
    strategy_id     TEXT,
    regime          TEXT,
    ev_30d          NUMERIC,
    ev_90d          NUMERIC,
    ev_slope        NUMERIC,     -- positive = improving, negative = decaying
    ev_weight       NUMERIC      -- 0.0 to 1.0, used by scanner optimizer
);
```

### `scanner_configs`

```sql
CREATE TABLE scanner_configs (
    id              UUID PRIMARY KEY,
    created_at      TIMESTAMPTZ,
    regime          TEXT,
    config_json     JSONB,
    optimizer_run_id TEXT
);
```

### `feature_distributions`

```sql
CREATE TABLE feature_distributions (
    computed_at     TIMESTAMPTZ,
    feature_name    TEXT,
    mean            NUMERIC,
    std_dev         NUMERIC,
    p25             NUMERIC,
    p75             NUMERIC
);
```

---

## Optimizer Validation Pipeline

Run before every optimizer training cycle:

```
scan_results
        │
        ▼
Timestamp Audit         data_as_of <= scanned_at? Flag violations.
        │
        ▼
Distribution Check      Current feature distributions vs training window.
                        Flag drift > 2 std deviations.
        │
        ▼
Regime Confidence Filter Drop rows where regime_confidence < 0.7
        │
        ▼
Fidelity Gate           Drop rows from high-drift periods per simulator_fidelity
        │
        ▼
EV Weight Join          Apply ev_weight from strategy_ev_weights
        │
        ▼
Clean weighted training dataset → Scanner Optimizer
```

---

## Build Order

| Phase | Component | Dependency |
|---|---|---|
| 1 | Simulator | None — build first, establishes ground truth |
| 2 | AI Scanner | Simulator must exist to generate labeled data |
| 3 | Fidelity Checker | Live trades + Simulator must both exist |
| 4 | Strategy Optimizer | Needs sim_outcomes volume (~500 rows minimum) |
| 5 | EV Tracker | Needs 4–6 weeks of live trade data |
| 6 | Scanner Optimizer | Needs sim_outcomes + ev_weights from EV Tracker |
| 7 | Recommendation Engine | Needs 6–12 months of EV history |

---

## Stack

| Component | Technology |
|---|---|
| Scanner + Optimizers | Python (XGBoost / scikit-learn) via gRPC from Go |
| Simulator | Go — deterministic, fast |
| Fidelity Checker | Go — deterministic comparison logic |
| EV Tracker | Go + PostgreSQL — rolling aggregations |
| Event log | EventStoreDB — trade events |
| Feature / result store | PostgreSQL |
| Orchestration | Temporal (Go SDK) — durable workflow per scan cycle |
| Config versioning | `scanner_configs` table with rollback by ID |
| Hosting | Hetzner |
| Monitoring | Prometheus + Loki + Grafana + OpenTelemetry |
| Watchdog | Healthchecks.io + UptimeRobot |
