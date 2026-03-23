# Plan: Hold-to-Expiration Credit Spread Strategy

## Problem

The current credit spread strategy exits positions early when the stock price "reverts" to the signal price (typically within 60 minutes). At that point, both options still carry significant time value, so buying back the short leg costs nearly as much as the original credit — resulting in ~$0 P&L on most trades. Meanwhile, max-loss exits dominate the realized P&L, producing a net loss despite 67%+ directional accuracy.

The expected_pl model correctly estimates that most spreads would be profitable **at expiration**, but the early-exit strategy never lets theta decay work.

## Evidence from backtester data

- **472 exit_reversion trades**: most have realized_pl ≈ 0 (exit price ≈ entry price)
- **~40 max_loss exits**: large negative P&L (-$1,000 to -$7,000 per spread)
- **Net result**: -$8,100 realized vs ~$210K expected (after double-count fix)
- **Median hold time**: ~60-90 minutes (should be 14-30 days for theta)

---

## Core Change: Remove Reversion Exit

In `_check_exits`, remove the reversion check (lines 917-973). The spread should be held until one of the risk management exits triggers, or the option expires.

```python
# REMOVE this block from _check_exits:
# reversion_complete = False
# if group.direction == "bullish" and high >= group.htf_signal_price:
#     reversion_complete = True
# ...
# if reversion_complete:
#     self._place_exit(group, entry, "reversion")
```

---

## Risk Management Framework

### Layer 1: Entry-Level Risk Controls (before placing the trade)

These gates prevent bad trades from entering the portfolio.

#### 1a. Minimum p_profit threshold

The current strategy enters whenever `expected_profit > 0`. With longer holds, be more selective:

```python
# New parameter
min_p_profit: float = 0.70  # only enter spreads with >=70% win probability

# In _place_entry, after computing p_profit:
if p_profit < self.min_p_profit:
    self.funnel["entries_skipped_low_p_profit"] += 1
    return
```

**Rationale**: At 70% win rate with a 5:1 max loss ratio (credit=$200, max_loss=$1,000), breakeven requires 71.4% win rate. A 70% minimum filters out marginal trades.

#### 1b. Minimum credit-to-width ratio

Require the net credit to be a meaningful fraction of the spread width:

```python
# New parameter
min_credit_width_ratio: float = 0.20  # credit >= 20% of spread width

# In _place_entry:
credit_width_ratio = net_credit_per_contract / actual_width
if credit_width_ratio < self.min_credit_width_ratio:
    self.funnel["entries_skipped_thin_credit"] += 1
    return
```

**Rationale**: A $5 wide spread collecting only $0.50 credit has a 10:1 risk/reward — catastrophic for hold-to-expiration. Requiring 20% ($1.00 on a $5 spread) ensures the credit meaningfully offsets risk.

#### 1c. Avoid earnings dates

Options experience massive IV crush after earnings. Selling spreads before earnings means:
- Entry: high IV inflates credit (good)
- Post-earnings: if stock moves against you, the move is sudden and unrecoverable

```python
# New parameter
earnings_blackout_days: int = 5

# In _place_entry, check if expiration straddles an earnings date:
# Skip entry if earnings announcement falls within the option's DTE window
```

**Implementation note**: Requires an earnings calendar (could use the existing Polygon integration or a simple hardcoded list for AAPL).

#### 1d. Maximum concurrent positions per expiration

Prevent concentration risk — don't stack too many spreads expiring on the same date:

```python
# New parameter
max_positions_per_expiration: int = 3

# Track: _positions_by_expiration: Dict[str, int]
# In _place_entry:
if self._positions_by_expiration.get(expiration_date, 0) >= self.max_positions_per_expiration:
    self.funnel["entries_skipped_expiration_concentration"] += 1
    return
```

**Rationale**: If 10 spreads all expire on the same Friday and the stock gaps against you, all 10 hit max loss simultaneously. Capping at 3 per expiration limits worst-case single-day loss.

---

### Layer 2: Position-Level Risk Controls (during the hold)

These exits protect individual positions from catastrophic loss.

#### 2a. Strike-breach stop-loss (primary)

Exit when the underlying price crosses the short strike. At this point the spread is ITM and likely heading to max loss — better to exit early and salvage remaining time value.

```python
# In _check_exits, FIRST check (before profit target):
if group.direction == "bullish":
    # Bull put: short put is higher strike
    if low <= entry.short_leg.strike:
        self._place_exit(group, entry, "strike_breach")
        continue
elif group.direction == "bearish":
    # Bear call: short call is lower strike
    if high >= entry.short_leg.strike:
        self._place_exit(group, entry, "strike_breach")
        continue
```

**Why this is better than the current max_loss check**: The current `spread_value >= max_loss_multiplier * credit` check relies on option mark-to-market, which can be stale or illiquid. The underlying price is always available and reliable.

**Refinement — breach + confirmation**: To avoid whipsaw on intraday touches, require the close (not just the low/high) to breach:

```python
close = bar_dict.get("close", 0.0)
if group.direction == "bullish" and close <= entry.short_leg.strike:
    self._place_exit(group, entry, "strike_breach")
```

#### 2b. Spread-value max loss (secondary, keep existing)

Keep the current `max_loss_multiplier` check as a backup in case the strike-breach doesn't trigger (e.g., gap moves where the stock jumps past the strike between candles):

```python
# Keep existing, but tighten the multiplier:
max_loss_multiplier: float = 1.5  # was 2.0

# Rationale: if spread value hits 1.5x credit, the position is deep ITM.
# At 2.0x, you've already lost more than the credit on the losing side.
```

#### 2c. Profit target (keep existing, adjust threshold)

```python
profit_target_pct: float = 0.50  # exit when 50% of max profit captured

# This means: if credit = $1.00/contract, exit when spread value drops to $0.50
# At 50% profit capture, the remaining $0.50 has diminishing returns
# vs the gamma risk of holding near expiration
```

For hold-to-expiration, consider a **time-based profit target** — take profit earlier if it comes quickly (the move went in your favor fast, lock it in):

```python
# In _check_exits:
days_held = (current_ts - entry.entry_timestamp).total_seconds() / 86400
dte_at_entry = entry.dte_at_entry  # new field on CreditSpreadEntry

# If >50% profit captured in first 25% of holding period, take it
if spread_value is not None:
    pct_time_elapsed = days_held / dte_at_entry if dte_at_entry > 0 else 1.0
    if pct_time_elapsed < 0.25 and spread_value <= 0.50 * credit_per_contract:
        self._place_exit(group, entry, "early_profit")
        continue
    # Standard profit target for later in the hold
    if spread_value <= (1 - self.profit_target_pct) * credit_per_contract:
        self._place_exit(group, entry, "profit_target")
        continue
```

**Rationale**: If 50% of the credit decays in the first 25% of the holding period, that's an unusually fast move. Taking profit preserves gains and frees collateral for new trades.

#### 2d. DTE-based exit (keep existing, tighten)

```python
time_decay_exit_dte: int = 3  # was 5

# Exit 3 days before expiration to avoid:
# - Pin risk (stock near strike on expiration day)
# - Assignment risk (ITM options auto-exercised)
# - Gamma blowup (delta changes rapidly near expiration)
```

#### 2e. Exit priority order

The order of exit checks matters — check the most critical first:

```
1. Strike breach (immediate risk — position is ITM)
2. Spread-value max loss (backup — catch gap moves)
3. Early profit target (fast wins — lock in gains)
4. Standard profit target (normal theta capture)
5. DTE exit (expiration approaching)
```

---

### Layer 3: Portfolio-Level Risk Controls (across all positions)

These prevent the portfolio from accumulating too much exposure.

#### 3a. Daily loss limit

Stop entering new trades if the portfolio has lost more than X% in a single day:

```python
# New parameter
daily_loss_limit_pct: float = 0.05  # stop new entries if down 5% today

# Track daily P&L:
# In _process_ltf_candle:
if self._daily_loss_pct() >= self.daily_loss_limit_pct:
    return  # skip all entry checks for rest of day
```

**Rationale**: A large adverse stock move can trigger multiple stop-losses simultaneously. Don't add new positions into a falling market.

#### 3b. Maximum simultaneous positions

Hard cap on total open spreads:

```python
# New parameter
max_open_positions: int = 10

# In _place_entry:
if self._count_open_positions() >= self.max_open_positions:
    self.funnel["entries_skipped_max_positions"] += 1
    return
```

**Rationale**: The current collateral limits (30% of equity) already constrain this, but a hard count prevents scenarios where many small spreads accumulate. With hold-to-expiration, positions stay open longer, so this becomes binding more often.

#### 3c. Directional balance limit

Prevent the portfolio from becoming too directionally biased:

```python
# New parameter
max_directional_imbalance: int = 3  # max 3 more bull than bear (or vice versa)

# Track:
bullish_count = sum(1 for g in active_groups if g.direction == "bullish")
bearish_count = sum(1 for g in active_groups if g.direction == "bearish")

# In _place_entry:
if group.direction == "bullish" and (bullish_count - bearish_count) >= self.max_directional_imbalance:
    self.funnel["entries_skipped_directional_imbalance"] += 1
    return
```

**Rationale**: If the portfolio is all bull put spreads and the stock drops sharply, every position loses simultaneously. Maintaining rough balance means some positions benefit from the move.

#### 3d. Per-expiration collateral limit

Extend the existing collateral tracking to be expiration-aware:

```python
# New parameter
max_collateral_per_expiration_pct: float = 0.10  # max 10% of equity per expiration date

# In _place_entry:
expiration_collateral = self._collateral_by_expiration.get(exp_date, 0)
if expiration_collateral + collateral_needed > equity * self.max_collateral_per_expiration_pct:
    self.funnel["entries_skipped_expiration_collateral"] += 1
    return
```

**Rationale**: Prevents catastrophic loss from all positions expiring on the same date. Even if total portfolio collateral is under 30%, having 25% expire on one Friday is dangerous.

#### 3e. Correlation-aware sizing

All positions are on AAPL — there's no diversification benefit. For single-underlying strategies, reduce total exposure:

```python
# Reduce max_total_collateral_pct from 30% to 20% for single-name
max_total_collateral_pct: float = 0.20
```

If the strategy expands to multiple underlyings, this can be relaxed.

---

### Layer 4: Expiration-Specific Controls

#### 4a. Close positions by DTE=1

Never hold into expiration day itself:

```python
# In _check_exits:
if dte <= 1:
    self._place_exit(group, entry, "pre_expiration")
    continue
```

**Rationale**: On expiration day, gamma is extreme (delta swings from 0 to 1 with small stock moves), assignment is automatic for ITM options, and the Go server's `postTickProcessing` handles expiration — but it may fill at unfavorable prices.

#### 4b. Widen stop as DTE decreases

Near expiration, spreads near the money are highly volatile. Tighten the exit trigger:

```python
# Dynamic strike-breach: as DTE decreases, exit further OTM
if dte <= 7:
    # Add a buffer — exit when stock is within 50% of the spread width of the short strike
    buffer = 0.5 * entry.spread_width
    if group.direction == "bullish" and close <= entry.short_leg.strike + buffer:
        self._place_exit(group, entry, "gamma_risk")
    elif group.direction == "bearish" and close >= entry.short_leg.strike - buffer:
        self._place_exit(group, entry, "gamma_risk")
```

**Rationale**: With 7 DTE and the stock near the short strike, the spread can swing from 20% loss to max loss in one candle. The buffer provides early warning.

---

## Updated Parameter Table

| Parameter | Current | Hold-to-Expiration | Rationale |
|-----------|---------|-------------------|-----------|
| `profit_target_pct` | 0.50 | 0.50 | Keep — take 50% of max profit |
| `max_loss_multiplier` | 2.0 | 1.5 | Tighten — exit sooner on losing trades |
| `time_decay_exit_dte` | 5 | 3 | Hold longer — more theta captured |
| `min_hold_candles` | 12 | 12 | Keep |
| `target_dte` | 30 | 21-30 | Shorter DTE = faster theta |
| `min_dte` | 14 | 14 | Keep — need liquidity |
| `max_collateral_pct` | 0.05 | 0.03 | Less per group — longer hold ties up capital |
| `max_total_collateral_pct` | 0.30 | 0.20 | Reduce — single underlying, all correlated |
| **NEW** `min_p_profit` | N/A | 0.70 | Only enter high-probability spreads |
| **NEW** `min_credit_width_ratio` | N/A | 0.20 | Require meaningful credit vs risk |
| **NEW** `max_open_positions` | N/A | 10 | Hard cap on simultaneous positions |
| **NEW** `max_directional_imbalance` | N/A | 3 | Prevent directional overconcentration |
| **NEW** `max_positions_per_expiration` | N/A | 3 | Prevent expiration concentration |
| **NEW** `daily_loss_limit_pct` | N/A | 0.05 | Stop trading after 5% daily drawdown |

---

## Exit Priority Order (updated _check_exits)

```python
def _check_exits(self, group, bar_dict):
    close = bar_dict.get("close", 0.0)
    high = bar_dict.get("high", 0.0)
    low = bar_dict.get("low", float("inf"))
    current_ts = bar_dict.get("datetime")

    for level_idx, entry in list(group.entries.items()):
        if level_idx in group.exited_entries:
            continue
        if entry.candles_held < self.min_hold_candles:
            continue

        dte = self._compute_dte(entry, current_ts)
        spread_value = self._compute_spread_value(entry)
        credit_per = entry.net_credit / (entry.contracts * 100)

        # 1. PRE-EXPIRATION: close by DTE=1, no exceptions
        if dte is not None and dte <= 1:
            self._place_exit(group, entry, "pre_expiration")
            continue

        # 2. STRIKE BREACH: underlying crossed short strike (close-based)
        if group.direction == "bullish" and close <= entry.short_leg.strike:
            self._place_exit(group, entry, "strike_breach")
            continue
        if group.direction == "bearish" and close >= entry.short_leg.strike:
            self._place_exit(group, entry, "strike_breach")
            continue

        # 3. GAMMA RISK: near-expiration + stock close to short strike
        if dte is not None and dte <= 7:
            buffer = 0.5 * entry.spread_width
            if group.direction == "bullish" and close <= entry.short_leg.strike + buffer:
                self._place_exit(group, entry, "gamma_risk")
                continue
            if group.direction == "bearish" and close >= entry.short_leg.strike - buffer:
                self._place_exit(group, entry, "gamma_risk")
                continue

        # 4. SPREAD-VALUE MAX LOSS: backup for gap moves
        if spread_value is not None:
            if spread_value >= self.max_loss_multiplier * credit_per:
                self._place_exit(group, entry, "max_loss")
                continue

        # 5. EARLY PROFIT: >50% profit in first 25% of holding period
        if spread_value is not None and entry.dte_at_entry and entry.dte_at_entry > 0:
            days_held = (current_ts - entry.entry_timestamp).total_seconds() / 86400
            pct_time = days_held / entry.dte_at_entry
            if pct_time < 0.25 and spread_value <= 0.50 * credit_per:
                self._place_exit(group, entry, "early_profit")
                continue

        # 6. STANDARD PROFIT TARGET
        if spread_value is not None:
            threshold = (1 - self.profit_target_pct) * credit_per
            if spread_value <= threshold:
                self._place_exit(group, entry, "profit_target")
                continue

        # 7. DTE EXIT: approaching expiration
        if dte is not None and dte <= self.time_decay_exit_dte:
            self._place_exit(group, entry, "time_decay")
            continue
```

---

## Implementation Order

1. Remove reversion exit from `_check_exits`
2. Add `strike_breach` exit (close-based, not high/low)
3. Add `gamma_risk` exit (DTE <= 7 + near strike)
4. Add `pre_expiration` exit (DTE <= 1)
5. Add entry-level gates: `min_p_profit`, `min_credit_width_ratio`, `max_positions_per_expiration`
6. Add portfolio-level gates: `max_open_positions`, `max_directional_imbalance`, `daily_loss_limit_pct`
7. Add `early_profit` exit logic
8. Adjust parameters: `max_loss_multiplier=1.5`, `max_total_collateral_pct=0.20`, etc.
9. Store `dte_at_entry` on `CreditSpreadEntry` for time-proportional exit logic
10. Run backtester on same date range and compare results
11. Iterate on parameter values based on backtester output

## New fields needed on CreditSpreadEntry

```python
@dataclass
class CreditSpreadEntry:
    # ... existing fields ...
    dte_at_entry: Optional[int] = None  # days to expiration at entry time
```

## New tracking state on CreditSpreadStrategy

```python
# In __init__:
self._positions_by_expiration: Dict[str, int] = {}  # exp_date → count
self._collateral_by_expiration: Dict[str, float] = {}  # exp_date → collateral
self._daily_pnl_start: Optional[float] = None  # equity at start of day
self._current_trading_day: Optional[date] = None
```

---

## Expected Outcome

With these risk controls in place:

- **Winners** (p_profit ~70%): capture full net credit ($150-400) via theta decay
- **Strike-breach losers** (~20%): exit early with partial loss, salvaging time value (~50-70% of max loss)
- **Max-loss losers** (~10%): gap moves that blow through the strike, full max loss

At 70% win rate, $200 avg credit, and $600 avg loss (blended):
- Expected P&L per trade = 0.70 × $200 - 0.30 × $600 = $140 - $180 = **-$40**

Still marginally negative — this means the strategy also needs:
- **Higher min_p_profit** (75-80%) to push win rate above breakeven
- **OR narrower spreads** ($2.50 width → lower max loss)
- **OR stricter credit-to-width ratio** (25%+) to ensure credit justifies the risk

The backtester will determine the right combination. The risk management framework above ensures that even suboptimal parameters can't produce catastrophic losses.
