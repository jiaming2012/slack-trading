# TradeSignal Integration Protocol

Standard for integrating TradeSignals into V2 strategies. Every strategy that detects signals and places orders MUST follow this protocol to ensure full traceability from signal detection through order execution.

## Architecture

```
Python Strategy                          Go Server
─────────────────                        ──────────────────
detect signal
  │
  ▼
write_signal() ──── WriteSignal RPC ───► globalSignalRepo
  │                                          │
  │ returns signal_id (UUID)                 ▼ fan-out (sim only)
  │                                      simSignalRepo
  ▼                                          │
store signal_id on TradeGroup                │
  │                                          ▼
  ▼                                      ReadPending (on NextTick)
place_order(signal_id=...) ─────────►    OrderRecord.SignalID
```

**Queryability:**
- `GetProcessedSignals(playground_id)` → all signals written during the sim
- `GetAccount(playground_id)` → all orders, each with `signal_id` linking back to the originating signal
- `SavePlayground` → persists both signals (to ESDB) and orders (to Postgres) with the link intact

## Signal Name Registry

Every strategy uses a registered signal name from `src/go/eventmodels/signal_name.go`. The compound signal key (e.g., `stochrsi_below_20|sma_50_cross_below`) goes in `attributes.signal_key`, NOT as the signal name.

| Strategy | Signal Name | Go Constant |
|----------|------------|-------------|
| MeanReversionStrategyV2 | `mean_reversion` | `SignalMeanReversion` |
| CoveredCallStrategyV2 | `covered_call` | `SignalCoveredCall` |
| CreditSpreadStrategyV2 | `mean_reversion` | `SignalMeanReversion` |
| OptionsMeanReversionV2 | `mean_reversion` | `SignalMeanReversion` |

To add a new signal name: add a constant to `signal_name.go` and include it in `Validate()`.

## Required Signal Attributes

When calling `self.playground.write_signal()`, include ALL of these attribute categories:

### 1. Signal Identity

| Attribute | Type | Description |
|-----------|------|-------------|
| `signal_key` | str | Compound key from `detect_atomic_signals_on_bar()`, e.g., `stochrsi_below_20\|sma_50_cross_below` |
| `htf_close` | str | HTF bar close price at signal detection |

### 2. PDF Lookup Context

| Attribute | Type | Description |
|-----------|------|-------------|
| `pdf_sample_size` | str | Number of historical observations for this compound signal |
| `pdf_ci_95_width` | str (6dp) | Measured 95% confidence interval width |
| `pdf_min_ci_width_threshold` | str (6dp) | Threshold the CI width was compared against |
| `pdf_sufficient_samples` | str | `"True"` if `ci_95_width < threshold` |

### 3. Horizon Stats (when horizon exists)

| Attribute | Type | Description |
|-----------|------|-------------|
| `horizon` | str | Which horizon was evaluated, e.g., `"1h"` |
| `horizon_mean` | str (6dp) | Expected forward return |
| `horizon_stddev` | str (6dp) | Forward return standard deviation |
| `horizon_model` | str | Return model name, e.g., `"bayesian_nig"` or `"empirical"` |
| `qualifies_for_group` | str | `"True"` if `horizon.mean > 0` (or strategy-specific gate) |

### 4. Decision Formula

| Attribute | Type | Description |
|-----------|------|-------------|
| `decision_formula` | str | Human-readable step-by-step rationale (see template below) |

**Template:**
```
1) detect_atomic_signals_on_bar(htf_bar, prev_bar) → [{signal_key}];
2) compound_key = sort+join → '{signal_key}';
3) pdf.get_signal('{signal_key}') → found={True/False}, sample_size={N};
4) sufficient_samples = ci_95_width({value}) < threshold({value}) → {True/False};
5) horizon['{horizon}'].mean({value}) > 0 → {True/False};
6) RESULT: CREATE GROUP at htf_close={price} | SKIP ({reason})
```

Strategies with different gating logic (e.g., options strategies checking IV) should extend steps 5-6 with their own criteria while preserving steps 1-4.

## Signal-to-Trade Linking

### Step 1: Capture signal_id from write_signal

```python
signal_id = self.playground.write_signal(
    name="mean_reversion",  # registered signal name
    symbol=self.symbol,
    timestamp=bar_ts,
    attributes=attrs,
)
sig["signal_id"] = signal_id
```

### Step 2: Store signal_id on TradeGroup (or equivalent)

The strategy's group/position data structure MUST have an `Optional[str]` field for `signal_id`:

```python
@dataclass
class TradeGroup:
    ...
    signal_id: Optional[str] = None  # UUID from WriteSignal RPC
```

Pass it through when creating the group:

```python
self._try_create_group(sig["signal_key"], bar_dict, pdf_entry, signal_id=sig.get("signal_id"))
```

### Step 3: Pass signal_id to every place_order call

ALL orders originating from this signal — entries, exits, stop-outs, end-of-sim closes — MUST include `signal_id`:

```python
self.playground.place_order(
    self.symbol,
    shares,
    OrderSide.BUY,
    "equity",
    level.price,
    attributes=attributes,
    signal_id=group.signal_id,  # links order back to signal
)
```

This applies to:
- **Entry orders** (deviation level fills)
- **Partial exit orders** (tier-based profit taking)
- **Stop-out orders** (HTF adverse close)
- **End-of-sim close orders** (cleanup)

### Step 4: Error handling

`write_signal()` failures MUST NOT break trading logic. Wrap in try/except and log a warning:

```python
try:
    signal_id = self.playground.write_signal(...)
    sig["signal_id"] = signal_id
except Exception as e:
    self.logger.warning(f"write_signal failed for {sig['signal_key']}: {e}")
```

If `write_signal` fails, `signal_id` will be `None` on the group. Orders still get placed — they just won't have the signal link. This is acceptable for sim resilience.

## Implementation Checklist

When migrating a strategy to V2 or creating a new strategy:

- [ ] Signal name is registered in `src/go/eventmodels/signal_name.go` and `Validate()`
- [ ] `write_signal()` called for every detected signal (before group/position creation)
- [ ] All required attributes included (signal identity, PDF context, horizon stats, decision formula)
- [ ] `signal_id` captured from `write_signal()` return value
- [ ] `signal_id` stored on the group/position data structure
- [ ] `signal_id` passed to ALL `place_order()` calls from that group
- [ ] `write_signal()` wrapped in try/except (failure does not block trading)
- [ ] Signal queryable via `GetProcessedSignals` after sim run
- [ ] Orders show `signal_id` in `GetAccount` response
- [ ] Signal + order data persists correctly via `SavePlayground`

## Reference Implementation

`src/clients/python/strategies/mean_reversion_v2.py` is the canonical example:
- Signal writing: `_process_htf_candle()` (lines 283-338)
- Signal-to-group link: `_try_create_group()` accepts `signal_id` parameter
- Order linking: `_place_entry()`, `_place_exit_tier()`, `_stop_out_group()`, `close_active_groups()`
