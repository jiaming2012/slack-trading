# Mean-Reversion Strategy: Architecture & Backtest Findings

## Strategy Overview

PDF-guided options mean-reversion strategy that buys calls on bullish dips and puts on bearish rallies. Uses forward-return distributions from historical compound signals to estimate reversion probability and expected profit.

### How It Works

1. **HTF signal detection** (1-hour bars): Detects compound signals (e.g., `bullish_pin_bar|stochrsi_below_20`) by combining candlestick patterns, StochRSI state, SuperTrend, and SMA crossovers.

2. **PDF lookup**: Each compound signal has a pre-built forward-return distribution (`PDFDocument`) across multiple horizons (1h, 4h, 1d, 2d, 1w, 2w). The mean return determines direction — positive mean = bullish, negative = bearish.

3. **Deviation levels**: `compute_deviation_levels()` places discrete entry price levels below (bullish) or above (bearish) the signal price using sigma bands from the forward-return distribution. Each level has a `p_revert` (probability the stock returns to signal price) and an allocated contract count via Kelly criterion.

4. **LTF entry triggers** (5-min bars): When the stock dips to a deviation level, the strategy fetches the options ladder and buys calls (bullish) or puts (bearish). Contract selection uses the `"model"` strike strategy, which anchors strikes ITM relative to signal price so that a successful reversion produces meaningful intrinsic value.

5. **Exit priority**:
   - **Profit target**: Option value >= 1.5x premium paid
   - **Reversion complete**: Stock touches signal price — sell at current option value
   - **Time decay**: DTE <= 3 days — force close to avoid expiration risk

### Key Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `max_premium_pct` | 0.02 | Max premium per group as % of equity |
| `total_contracts_per_group` | 10 | Contracts allocated across all levels |
| `target_dte` | 14 | Target days to expiration |
| `min_dte` / `max_dte` | 5 / 30 | DTE range for contract selection |
| `profit_target_pct` | 0.50 | Exit when option value >= 1.5x premium |
| `time_decay_exit_dte` | 3 | Force exit when DTE <= this |
| `min_expected_profit` | 50.0 | Minimum EV to allow entry |
| `min_hold_candles` | 6 | Minimum 5-min bars before exit allowed |
| `strike_strategy` | "model" | ITM anchor based on sigma distance |
| `long_only` | false | Skip bearish signals when true |
| `tail_threshold` | 0.005 | Stop generating levels when < 0.5% of returns reached |

---

## Expected Profit Model

### Evolution

**Original (binary model)**:
```
E[profit] = p_revert_bounded * intrinsic_at_signal - total_premium
```
Binary outcome: stock either fully reverts (yielding intrinsic) or doesn't (yielding 0). Misses partial reversion, overshoot, and tail behavior.

**Current (distribution-integrated model)**:
```python
exit_prices = signal_price * (1 + forward_returns)   # one per historical observation
intrinsics  = max(0, exit_price_i - strike)           # call; reverse for put
E[profit]   = mean(intrinsics) * 100 * contracts - total_premium
```
Iterates over every forward return in the PDF horizon matched to the option's DTE. Captures partial reversion (stock goes halfway back — call still has intrinsic), overshoot (stock passes signal — extra value), and the full distribution shape.

### DTE-Matched Horizon Selection

Instead of always using the longest PDF horizon, `_select_horizon_for_dte()` picks the one whose calendar-day equivalent best matches the option's DTE:

| Horizon | Calendar Days | Best for DTE |
|---------|:---:|:---:|
| 1h | ~0.15 | < 1 day |
| 4h | ~0.62 | < 1 day |
| 1d | 1.0 | 1-2 days |
| 2d | 2.0 | 2-5 days |
| 1w | 5.0 | 5-10 days |
| 2w | 10.0 | 10+ days |

Falls back to longest available horizon if no exact match.

### Why Intrinsic-Only (No Black-Scholes)

The model uses intrinsic value only — no time value estimation. This is intentional:

- **Conservative lower bound**: True `E[option_value] >= intrinsic`. If an entry passes this filter, it's very likely profitable including time value.
- **Exit IV is unknowable**: Black-Scholes requires estimating exit implied volatility, which depends on market conditions at exit time. Incorrect IV assumptions could make the estimate *worse*.
- **Complexity vs. payoff**: BS requires modeling IV surface evolution — high complexity for low-to-moderate accuracy gain, since time value decays rapidly as DTE approaches 0.

### Reversion Probability Metrics

Two p_revert values are tracked per entry:

- **`p_revert_unbounded`**: From deviation levels — `P(return >= -dip_magnitude)` using all forward returns. Measures whether the stock *ever* reaches signal price, regardless of when.
- **`p_revert_bounded`**: From `_compute_time_bounded_p_revert()` — `P(return >= 0)` at the longest horizon. Measures whether the stock actually reverts *within the PDF's observation window*.

---

## Backtest Results & Analysis

### Dataset

- **Symbol**: AAPL
- **Period**: ~1 year of 5-min/1-hour data
- **928 order rows**: 466 entries, 462 exits
- **Exit types**: 285 reversion, 120 time decay, 57 profit target
- **No stop-outs**: Options premium is the max loss (no stop needed)

### Headline Numbers

| Metric | Value |
|--------|-------|
| Total expected P&L | +$148,000 |
| Total realized P&L | -$20,000 |
| Model overestimated by | ~$168,000 |

**The distribution-based EV model is not predictive of realized P&L.**

### P&L Decomposition by Expected Profit Bucket

| EV Bucket | Exits | Reversion P&L | Time Decay P&L | Profit Target P&L | **Total** | TD Rate |
|-----------|:---:|---:|---:|---:|---:|:---:|
| **0-100** | 155 | +$28,200 | -$39,571 | +$25,887 | **+$14,516** | 23% |
| 101-300 | 144 | +$16,650 | -$47,640 | +$15,076 | -$15,914 | 31% |
| 301-600 | 81 | +$9,847 | -$28,917 | +$10,135 | -$8,935 | 30% |
| 601-1000 | 57 | +$7,323 | -$12,470 | +$2,730 | -$2,417 | 16% |
| 1001-1500 | 18 | -$1,296 | -$9,761 | $0 | -$11,057 | 33% |
| 1501-2500 | 7 | $0 | -$1,372 | +$5,056 | +$3,684 | 14% |

**The lowest EV bucket (0-100) is the only consistently profitable one.**

### Five Key Findings

#### 1. Time Decay Is the Dominant Loss Driver

Every single time-decay exit loses money — averaging **-$1,100 per trade** regardless of bucket. This is roughly 75-100% of the entry premium. When the option doesn't revert fast enough and hits DTE <= 3 days, essentially the entire premium is lost.

**Every bucket makes money on reversion exits. Every bucket loses money on time decay exits.** The net depends on the ratio between them.

#### 2. The Model Cannot Distinguish Trades That Will Time-Decay

Within each bucket, expected P&L is virtually identical for trades that end up reverting vs. time-decaying:

| Bucket | Time Decay Avg Expected | Reversion Avg Expected |
|--------|:-:|:-:|
| 0-100 | $69 | $71 |
| 101-300 | $174 | $179 |
| 301-600 | $414 | $444 |

The model assigns the same EV to trades that succeed and trades that fail. **It cannot distinguish them** because it only models *where* the stock will be at horizon, not *how fast* it gets there.

#### 3. Premiums Are Flat Across Buckets — The Market Already Prices Reversion

| Bucket | Avg Entry Premium | Avg Total Cost | p_revert_bounded |
|--------|:-:|:-:|:-:|
| 0-100 | $4.17 | $1,500 | 0.496 |
| 101-300 | $4.14 | $1,560 | 0.516 |
| 301-600 | $4.38 | $1,543 | 0.577 |
| 601-1000 | $4.53 | $1,761 | 0.607 |
| 1001-1500 | $4.42 | $1,956 | 0.607 |

High-EV entries have **higher p_revert** (0.60 vs 0.50) but pay nearly the same premium (~$4/contract). The options market has already priced the higher reversion probability into the premium via implied volatility. **The model's edge is already captured by the market.**

#### 4. Low-EV Entries Win Because They Revert Faster

| Bucket | Reversion Win Rate | Reversion $0 Rate | Profit Target Hit Rate |
|--------|:-:|:-:|:-:|
| **0-100** | **52%** | 32% | **17%** |
| 101-300 | 38% | 49% | 15% |
| 301-600 | 21% | 66% | 5% |
| 601-1000 | 19% | 49% | 2% |
| 1001-1500 | 17% | 50% | 0% |

The 0-100 bucket has:
- **2.5x the reversion win rate** of higher buckets
- **The highest profit target hit rate** (17% vs 2% or 0%)
- **The lowest time decay rate** (23% vs 30-33%)

These are cheap, fast-reverting trades. They enter, the stock pops back quickly, and the option exits with profit before theta eats the premium.

#### 5. Reversion Exits With $0 P&L Reveal the IV Crush Problem

134 out of 285 reversion exits (47%) have **$0 realized P&L** — the stock reverted to signal price, but the option was worth exactly what was paid for it. This happens because:
- The stock reverted *slowly enough* that time decay offset the intrinsic gain
- IV crushed after the initial signal event, reducing extrinsic value
- The option's value at exit ≈ entry premium despite favorable stock movement

This $0-reversion rate is worst in higher EV buckets (66% for 301-600 vs 32% for 0-100), confirming that high-EV entries take longer to play out.

### Duration-P&L Correlation

Overall correlation between hold duration and realized P&L: **r = -0.33** (negative — longer holds lose more). By bucket:

| Bucket | Duration-P&L Correlation | Interpretation |
|--------|:-:|---|
| 0-100 | -0.36 | Moderate negative |
| 101-300 | -0.42 | Moderate negative |
| 1001-1500 | **-0.81** | Very strong negative |
| 301-600 | -0.28 | Weak negative |
| 601-1000 | -0.14 | Weak negative |

For options, **time is the enemy**. The longer you hold, the more you lose. This is fundamentally different from the stock strategy where holding costs nothing.

### Winners vs. Losers Within Buckets

| Bucket | Winner Avg Duration | Loser Avg Duration | Winner Avg P&L | Loser Avg P&L |
|--------|:-:|:-:|:-:|:-:|
| 0-100 | 7,056 min | 7,391 min | +$794 | -$546 |
| 101-300 | 6,913 min | 6,743 min | +$777 | -$514 |
| 301-600 | 8,880 min | 5,974 min | +$1,417 | -$457 |
| 601-1000 | 7,835 min | 3,828 min | +$1,290 | -$326 |
| 1001-1500 | 2,880 min | 6,815 min | +$212 | -$718 |

In higher buckets, **losers are held much longer than winners** (especially 1001-1500: losers held 6,815 min vs winners at 2,880 min). The winners that exist in high-EV buckets are the rare ones that revert quickly.

---

## Failed Hypothesis: EV-Based Volume Scaling

### Original Plan (Phase 2)

Scale contract allocation based on expected profit per contract:
```python
ev_ratio = ev_per_contract / baseline_ev
scale = clamp(ev_ratio, 0.5, 2.0)
contracts = round(level.shares * scale)
```

### Go/No-Go Verdict: FAIL

The backtest proves this would **amplify losses**, not profits:
- Higher EV entries lose money (-$8K to -$16K per bucket)
- Scaling into them would concentrate capital in the worst-performing trades
- The only profitable bucket (0-100) would get *less* allocation
- No positive correlation exists between expected and realized P&L

---

## Structural Insights for Options vs. Stock Strategies

### Why Options Behave Differently From Stocks

| Factor | Stock Strategy | Options Strategy |
|--------|---------------|-----------------|
| **Holding cost** | Zero (no theta) | ~$170/day per contract (theta decay) |
| **Time to profit** | Can wait indefinitely | Must revert before DTE |
| **Loss on slow reversion** | Still profits | Loses to time decay |
| **Market pricing of reversion** | Stock price is stock price | IV already embeds reversion probability into premium |
| **Partial reversion** | Linear profit | Non-linear (intrinsic + IV effects) |
| **Max loss** | Stop-out level | Premium paid (bounded) |

### The Core Problem

The options market is more efficient at pricing mean-reversion than the stock market. When a stock dips 2σ below its signal price:
- **Stock**: Buy shares, wait for reversion, profit = `(signal - entry) × shares`
- **Options**: The options chain already reflects the reversion probability in its IV. A high p_revert signal means higher IV → higher premium → less edge

This is why the model's expected profit doesn't translate to realized profit — **the edge is in the premium, not in the model**.

### What Actually Works: Cheap, Fast Trades

The 0-100 bucket succeeds because:
1. **Low premium** ($4/contract, $1,500 total) — small amount at risk
2. **Fast reversion** — stock bounces back quickly, option profits before theta bites
3. **High profit target hit rate** (17%) — quick spikes lock in gains
4. **Low time decay rate** (23%) — fewer trades languish until expiration

These are fundamentally different from high-EV entries, which represent scenarios where the model says "big potential upside" but the stock takes too long to get there.

---

## Stock Strategy Backtest Results

### Stock Strategy Overview

The stock mean-reversion strategy shares the same PDF signal detection and deviation levels but trades shares instead of options:
- **Entry**: Buy shares at σ-deviation bands below signal price
- **Exit**: Partial exits across 3 tiers as price recovers toward signal
- **Stop**: Sell all remaining shares if HTF close breaches 95th percentile adverse move
- **No time decay**: Holding costs nothing (beyond margin opportunity cost)

### Stock EV Formula (Binary Model)

```python
avg_exit_on_revert = weighted_average(tier_exit_prices)   # from partial exit tiers
expected_exit = avg_exit_on_revert * p_revert + stop_price * (1 - p_revert)
expected_profit = shares * (expected_exit - entry_price)
```

Binary: stock either fully reverts (exits across 3 tiers at graduated prices) or stops out (all shares sold at stop price). No intermediate outcomes modeled.

### Dataset

- **Symbol**: AAPL
- **Period**: 2025-06-01 to 2026-02-28 (weekly PDF retrain, Bayesian NIG model)
- **732 order rows**: 256 entries, 476 exits (426 partial exits, 50 stop-outs)
- **334 trade groups**: 132 closed, 129 stopped out, 73 pending (no entry triggered)

### Headline Numbers

| Metric | Stock | Options |
|--------|:---:|:---:|
| Total expected P&L | $137,176 | $148,000 |
| Total realized P&L | $111,946 | -$20,000 |
| Prediction error | **-8%** (underestimated) | **+840%** (overestimated) |
| Directional accuracy | **86.3%** | ~30% |
| Return on capital | **+93.0%** | Negative |
| Win rate (exit level) | 87% (413W / 63L) | ~40% |

**The stock binary EV model is dramatically more accurate than the options distribution model.**

### EV Bucket Performance (Group Level)

| Bucket | Groups | Avg Expected | Avg Realized | Ratio | Win Rate | Stop Rate |
|--------|:---:|:---:|:---:|:---:|:---:|:---:|
| 0-100 | 7 | $32 | $126 | 3.95x | 100% | 29% |
| 101-300 | 22 | $185 | $851 | 4.61x | 73% | 41% |
| 301-600 | 77 | $539 | $581 | 1.08x | 86% | 17% |
| 601-1000 | 78 | $736 | $611 | 0.83x | 77% | 33% |

Key observations:
- **All buckets are profitable** (unlike options where only 0-100 made money)
- Low-EV entries (0-100, 101-300) **massively outperform** expectations (3-5x)
- High-EV entries (601-1000) **slightly underperform** (0.83x) but still profitable
- Overall group-level correlation is near zero (r = -0.042)

### Why the Binary Model Works for Stocks

1. **No theta decay**: 87% of exits are profitable partial exits. Compare to options where 26% were total-loss time-decay exits.

2. **Most groups fully revert**: Even in the 601-1000 bucket, 69% of groups completed all 3 exit tiers. The binary assumption (full reversion or stop) is mostly correct.

3. **Overshoot drives underestimation**: Low-EV entries outperform 3-5x because the stock often overshoots the signal price. The binary model caps reversion profit at the signal price tier exits — it can't capture overshoot.

4. **No IV pricing of reversion**: Stock prices don't embed reversion probability. A stock at $195 after a dip from $200 is just $195 — unlike options where IV already prices the bounce.

### Prediction Error Analysis

| Bucket | MAE | Mean Error | Overestimated | Underestimated |
|--------|:---:|:---:|:---:|:---:|
| 0-100 | $101 | +$94 | 29% | 71% |
| 101-300 | $1,080 | +$666 | 36% | 64% |
| 301-600 | $554 | +$42 | 49% | 51% |
| 601-1000 | $610 | -$126 | 60% | 40% |

- **Low-EV buckets are systematically underestimated** (model says $32, realizes $126) — these are overshoot cases
- **High-EV buckets are slightly overestimated** (model says $736, realizes $611) — higher stop-out rate than predicted
- Overall MAE is $623 per group with std $1,019

### Stock Strategy Bottleneck: Stop-Out Rate

The biggest P&L drag is stop-outs, not EV misprediction:

| Bucket | Stop Rate | Stop-Out Impact |
|--------|:---:|---|
| 0-100 | 29% | Small sample (2/7 groups) |
| 101-300 | **41%** | Highest rate — shallow dips stop out easily |
| 301-600 | **17%** | Best stop rate — sweet spot |
| 601-1000 | **33%** | Deep dips have wide stops, but more adverse moves reach them |

129 of 334 groups (39%) stopped out. Each stop-out loses the full distance from entry to stop price. Reducing stop-out rate — especially in 101-300 and 601-1000 — is the highest-leverage improvement.

### Tier Completion by Bucket

| Bucket | Avg Tiers Triggered | Groups With All 3 Tiers |
|--------|:---:|:---:|
| 0-100 | 3.0 | 5/5 (100%) |
| 101-300 | 2.8 | 13/16 (81%) |
| 301-600 | 2.8 | 61/69 (88%) |
| 601-1000 | 2.6 | 47/68 (69%) |

Higher EV entries complete fewer tiers before stopping out — the stock enters deeper but the reversion is less reliable.

### Duration-P&L Correlation (Stock)

Unlike options (r = -0.33), stock duration-P&L correlation is **not meaningfully negative** — holding longer doesn't systematically hurt because there's no theta.

### Transferability of Options Findings to Stocks

| Finding | Options Impact | Applies to Stocks? |
|---------|---------------|:---:|
| Distribution integration | Replaced binary model | **Partially** — would capture overshoot in low-EV entries but path-dependent tiered exits limit accuracy |
| DTE-matched horizon | Critical for options | **No** — stocks have no expiration |
| IV prices reversion into premium | Fatal flaw | **No** — stock prices don't embed p_revert |
| Time decay kills slow trades | -$1,100/trade | **No** — no holding cost for stocks |
| EV-based volume scaling | Failed (Phase 2) | **Not tested** — would likely work better since all buckets are profitable |

---

## Recommendations for Next Steps

### Immediate (Filter Improvements)

1. **Max expected profit cap**: Set an upper bound on `min_expected_profit` to avoid high-EV entries that tend to lose. Based on the data, entries with expected_pl > $300 have poor realized outcomes.

2. **Time-to-reversion filter**: If the PDF can estimate *speed* of reversion (e.g., using shorter horizons like 1d or 2d instead of 2w), filter entries where the probability of reverting within 2-3 days is low.

3. **Tighter time decay exit**: Currently exits at DTE <= 3. Consider exiting earlier (DTE <= 5) to reduce the average loss per time-decay exit.

### Medium-Term (Model Changes)

4. **Premium-adjusted EV**: Divide expected profit by premium paid to get a return ratio. The current model treats a $100 expected profit on $1,500 premium the same as $100 on $500 premium — but the latter is 3x better risk/reward.

5. **Speed-weighted reversion probability**: Weight forward returns by *when* they occurred (early returns in the window count more for options since theta is working against you).

6. **Separate entry filter for stocks vs options**: The same PDF serves both strategies, but the optimal filter criteria differ. Stock strategy can use total EV; options strategy should use premium-relative EV and short-horizon reversion probability.

### Long-Term (Research)

7. **IV-adjusted expected profit**: Instead of intrinsic-only, model the relationship between signal sigma distance and IV premium empirically. If entries at 2σ always have 40% IV markup, discount the expected profit accordingly.

8. **Path-dependent simulation**: Monte Carlo simulation of stock price paths (not just terminal distribution) to estimate *when* reversion happens and how much theta is lost along the way.

---

## File Reference

| File | Role |
|------|------|
| `options_mean_reversion_strategy.py` | Main strategy: signal detection, entry/exit logic, EV computation |
| `mean_reversion_strategy.py` | Stock version of the strategy (no theta concerns) |
| `deviation_levels.py` | Sigma-band level placement and contract allocation |
| `pdf_types.py` | `HorizonStats`, `SignalPDF`, `PDFDocument` data structures |
| `pdf_builder.py` | Builds PDFs from historical bar data |
| `return_models.py` | `EmpiricalModel`, `BayesianNIGModel` for forward returns |
| `risk_management.py` | Kelly criterion position sizing |
| `partial_exit_manager.py` | Tier-based partial exit logic |
| `mean_reversion_report.py` | Post-simulation analysis and CSV generation |
| `backtester_playground_client_grpc.py` | Twirp RPC client for backtester |
| `test_options_mean_reversion_strategy.py` | Unit tests (55 tests) |
| `test_mean_reversion_strategy.py` | Stock strategy tests (38 tests) |
