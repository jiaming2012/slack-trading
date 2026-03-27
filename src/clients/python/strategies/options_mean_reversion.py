"""
Options Mean-Reversion Strategy.

Multi-timeframe strategy trading options instead of shares:
- HTF (1-hour): detects bullish/bearish compound signals via PDF
- LTF (5-min): buys calls on dips (bullish) or puts on rallies (bearish)
- Scales in at σ-deviation bands where P(revert) is higher
- Risk capped at premium paid — no stop-out needed

Both directions, single-symbol, configurable strike selection.
"""

from __future__ import annotations

import uuid
from dataclasses import dataclass, field
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Set

import numpy as np

from loguru import logger as _default_logger

from engine.client import (
    BacktesterPlaygroundClient,
    CreatePolygonPlaygroundRequest,
    PlaygroundEnvironment,
    Repository,
    RepositorySource,
)
from lib.deviation_levels import DeviationPlan, compute_deviation_levels
from lib.pdf_builder import detect_atomic_signals_on_bar, _get, _get_dt
from lib.pdf_types import HorizonStats, PDFDocument, SignalPDF
from engine.types import OrderSide
from rpc.playground_pb2 import GetOptionsLadderRequest
from strategies.base_strategy import BaseStrategy


# ------------------------------------------------------------------ #
# Data structures
# ------------------------------------------------------------------ #

@dataclass
class OptionsContractEntry:
    """One options position opened at a deviation level."""

    contract_symbol: str          # OCC format: O:AAPL251205C00235000
    option_type: str              # "call" or "put"
    strike: float
    contracts: int
    premium_per_contract: float   # ask price at entry
    total_premium: float          # contracts * premium * 100
    level_index: int
    sigma_distance: float
    p_revert: float
    entry_stock_price: float
    expiration_date: str
    candles_held: int = 0         # LTF candles since entry (for min hold)


@dataclass
class OptionsTradeGroup:
    """One group of options trades spawned by a single HTF signal."""

    group_id: str
    htf_signal_key: str
    htf_signal_price: float
    htf_signal_timestamp: datetime
    direction: str                          # "bullish" or "bearish"
    deviation_plan: DeviationPlan
    entries: Dict[int, OptionsContractEntry] = field(default_factory=dict)
    exited_entries: Set[int] = field(default_factory=set)
    status: str = "pending"                 # pending, active, closed
    total_premium_spent: float = 0.0
    model_name: str = "empirical"
    htf_stddev: float = 0.0                 # stddev from HTF horizon (for dip_magnitude)
    horizons: Dict[str, HorizonStats] = field(default_factory=dict)  # all PDF horizons


# ------------------------------------------------------------------ #
# Strategy class
# ------------------------------------------------------------------ #

class OptionsMeanReversionStrategy(BaseStrategy):
    """
    PDF-guided options mean-reversion strategy.

    Buys calls on bullish dips, puts on bearish rallies.
    Risk per group is capped at premium paid.
    """

    def __init__(
        self,
        playground: BacktesterPlaygroundClient,
        symbol: str,
        logger=None,
        pdf: PDFDocument = None,
        max_premium_pct: float = 0.02,
        stop_percentile: float = 0.95,
        total_contracts_per_group: int = 0,
        htf_horizon: str = "1h",
        target_dte: int = 14,
        min_dte: int = 5,
        max_dte: int = 30,
        profit_target_pct: float = 0.50,
        time_decay_exit_dte: int = 3,
        strike_strategy: str = "model",
        min_hold_candles: int = 6,
        tail_threshold: float = 0.005,
        min_expected_profit: float = 50.0,
        long_only: bool = False,
    ):
        super().__init__(playground, symbol, logger or _default_logger)
        self.pdf = pdf
        self.max_premium_pct = max_premium_pct
        self.stop_percentile = stop_percentile

        # Auto-size contracts from balance when not specified.
        # Rough estimate: each contract costs ~3% of stock price * 100 shares.
        if total_contracts_per_group <= 0:
            ltf_bar = playground.current_candles.get(symbol, {}).get(playground.ltf_seconds)
            current_price = ltf_bar.close if ltf_bar else 0
            if current_price > 0:
                est_premium_per_contract = current_price * 0.03 * 100
                budget = playground.account.balance * max_premium_pct
                total_contracts_per_group = max(1, int(budget / est_premium_per_contract))
            else:
                total_contracts_per_group = 1
            self.logger.info(
                f"Auto-sized contracts/group: {total_contracts_per_group}"
                f" (budget ${playground.account.balance * max_premium_pct:,.2f})"
            )

        self.total_contracts_per_group = total_contracts_per_group
        self.htf_horizon = htf_horizon
        self.target_dte = target_dte
        self.min_dte = min_dte
        self.max_dte = max_dte
        self.profit_target_pct = profit_target_pct
        self.time_decay_exit_dte = time_decay_exit_dte
        self.strike_strategy = strike_strategy
        self.min_hold_candles = min_hold_candles
        self.tail_threshold = tail_threshold
        self.min_expected_profit = min_expected_profit
        self.long_only = long_only

        # State
        self.trade_groups: List[OptionsTradeGroup] = []
        self._prev_htf_bar: Optional[dict] = None
        self._prev_ltf_bar: Optional[dict] = None
        self._htf_signal_dedup: Set[str] = set()
        self._cached_ladder = None
        self._cached_ladder_tick_ts = None

        # Funnel counters
        self.funnel = {
            "htf_bars": 0,
            "ltf_bars": 0,
            "signals_detected": 0,
            "bullish_signals": 0,
            "bearish_signals": 0,
            "groups_created": 0,
            "groups_skipped_empty_plan": 0,
            "groups_skipped_dedup": 0,
            "entries_placed": 0,
            "entries_skipped_no_contract": 0,
            "entries_skipped_budget": 0,
            "entries_skipped_wide_spread": 0,
            "entries_skipped_expected_profit": 0,
            "exits_profit": 0,
            "exits_reversion": 0,
            "exits_time_decay": 0,
            "exits_end_of_sim": 0,
            "groups_closed": 0,
            "ladder_fetches": 0,
            "ladder_empty": 0,
        }

    # ------------------------------------------------------------------ #
    # Repository configuration
    # ------------------------------------------------------------------ #

    @classmethod
    def get_repositories(cls, symbol: str) -> List[Repository]:
        """5-min LTF + 1-hour HTF with indicators."""
        indicators = [
            "supertrend", "stochrsi", "atr", "doji", "hammer",
            "sma_50", "sma_100", "sma_200",
            "stochrsi_cross_above_20", "stochrsi_cross_below_80",
        ]
        return [
            Repository(
                symbol=symbol,
                timespan_multiplier=5,
                timespan_unit="minute",
                indicators=indicators,
                history_in_days=365,
            ),
            Repository(
                symbol=symbol,
                timespan_multiplier=1,
                timespan_unit="hour",
                indicators=indicators,
                history_in_days=365,
            ),
        ]

    # ------------------------------------------------------------------ #
    # Main processing
    # ------------------------------------------------------------------ #

    def process_candles(self, new_candles) -> None:
        """Process new candles from a tick delta."""
        self._cached_ladder = None
        self._cached_ladder_tick_ts = None

        for c in new_candles:
            if not hasattr(c, "period") or not hasattr(c, "symbol"):
                continue
            if c.symbol != self.symbol:
                continue

            bar_dict = _bar_to_dict(c.bar)

            if c.period == self.playground.htf_seconds:
                self._process_htf_candle(bar_dict)
            elif c.period == self.playground.ltf_seconds:
                self._process_ltf_candle(bar_dict)

    def _process_htf_candle(self, bar_dict: dict) -> None:
        """Handle a completed HTF candle."""
        self.funnel["htf_bars"] += 1

        # 1. Detect signals
        signals = detect_atomic_signals_on_bar(
            bar_dict, self._prev_htf_bar, timeframe="ltf",
        )
        self._prev_htf_bar = bar_dict

        if signals and self.pdf:
            key = "|".join(sorted(signals))
            pdf_entry = self.pdf.get_signal(key)
            if pdf_entry and pdf_entry.sufficient_samples:
                horizon = pdf_entry.horizons.get(self.htf_horizon)
                if horizon:
                    if horizon.mean > 0:
                        self._try_create_group(key, bar_dict, pdf_entry, "bullish")
                    elif horizon.mean < 0 and not self.long_only:
                        self._try_create_group(key, bar_dict, pdf_entry, "bearish")

        # No stop evaluation — premium is the max loss

    def _process_ltf_candle(self, bar_dict: dict) -> None:
        """Handle a completed LTF candle."""
        self.funnel["ltf_bars"] += 1
        self._prev_ltf_bar = bar_dict

        low = bar_dict.get("low", float("inf"))
        high = bar_dict.get("high", 0.0)

        # Increment hold counter for all open entries
        for group in self.trade_groups:
            if group.status not in ("pending", "active"):
                continue
            for idx, entry in group.entries.items():
                if idx not in group.exited_entries:
                    entry.candles_held += 1

        groups_with_new_fills: set = set()

        # Check entries
        for group in self.trade_groups:
            if group.status not in ("pending", "active"):
                continue
            fills_before = len(group.entries)
            self._check_entries(group, bar_dict)
            if len(group.entries) > fills_before:
                groups_with_new_fills.add(id(group))

        # Check exits (skip groups that just filled this candle)
        for group in self.trade_groups:
            if group.status != "active":
                continue
            if id(group) in groups_with_new_fills:
                continue
            self._check_exits(group, bar_dict)

    # ------------------------------------------------------------------ #
    # Group creation
    # ------------------------------------------------------------------ #

    def _try_create_group(
        self, signal_key: str, bar_dict: dict, pdf_entry: SignalPDF,
        direction: str,
    ) -> None:
        """Attempt to create a new options trade group."""
        self.funnel["signals_detected"] += 1
        if direction == "bullish":
            self.funnel["bullish_signals"] += 1
        else:
            self.funnel["bearish_signals"] += 1

        timestamp = bar_dict.get("datetime", datetime.now())
        if isinstance(timestamp, str):
            from dateutil.parser import parse as parse_dt
            timestamp = parse_dt(timestamp)

        # Dedup
        dedup_key = f"{timestamp.isoformat()}|{signal_key}|{direction}"
        if dedup_key in self._htf_signal_dedup:
            self.funnel["groups_skipped_dedup"] += 1
            return
        self._htf_signal_dedup.add(dedup_key)

        signal_price = bar_dict.get("close", 0.0)
        horizon = pdf_entry.horizons.get(self.htf_horizon)
        if not horizon:
            return

        # Budget: premium-based (we'll enforce at entry time when we know the ask)
        # Use a large max_loss_budget so deviation_levels doesn't truncate
        # (budget enforcement is done per-entry based on actual premium)
        equity = self.playground.account.equity
        max_loss_budget = equity  # no truncation needed for options

        # For bearish signals, we need to negate returns for deviation_levels
        # since it assumes levels below signal price
        if direction == "bearish":
            forward_returns = [-r for r in horizon.forward_returns]
        else:
            forward_returns = horizon.forward_returns

        plan = compute_deviation_levels(
            signal_price=signal_price,
            stddev=horizon.stddev,
            forward_returns=forward_returns,
            stop_percentile=self.stop_percentile,
            max_loss_budget=max_loss_budget,
            total_shares=self.total_contracts_per_group,
            tail_threshold=self.tail_threshold,
        )

        if not plan.levels:
            self.funnel["groups_skipped_empty_plan"] += 1
            self.logger.debug(
                f"Empty deviation plan for {direction} signal {signal_key}"
                f" at ${signal_price:.2f}"
            )
            return

        # For bearish: mirror levels above signal price
        if direction == "bearish":
            for level in plan.levels:
                level.price = signal_price + (signal_price - level.price)

        group = OptionsTradeGroup(
            group_id=str(uuid.uuid4())[:8],
            htf_signal_key=signal_key,
            htf_signal_price=signal_price,
            htf_signal_timestamp=timestamp,
            direction=direction,
            deviation_plan=plan,
            htf_stddev=horizon.stddev,
            horizons=pdf_entry.horizons,
        )

        self.trade_groups.append(group)
        self.funnel["groups_created"] += 1
        self.logger.info(
            f"New {direction} options group {group.group_id}: signal={signal_key}"
            f" price=${signal_price:.2f} levels={len(plan.levels)}"
        )

    # ------------------------------------------------------------------ #
    # Entry checks
    # ------------------------------------------------------------------ #

    def _check_entries(self, group: OptionsTradeGroup, bar_dict: dict) -> None:
        """Check if any unfilled levels are breached."""
        low = bar_dict.get("low", float("inf"))
        high = bar_dict.get("high", 0.0)

        for idx, level in enumerate(group.deviation_plan.levels):
            if idx in group.entries or idx in group.exited_entries:
                continue

            triggered = False
            if group.direction == "bullish" and low <= level.price:
                triggered = True
            elif group.direction == "bearish" and high >= level.price:
                triggered = True

            if triggered:
                self._place_entry(group, idx, level, bar_dict)

    def _place_entry(
        self, group: OptionsTradeGroup, level_idx: int, level, bar_dict: dict,
    ) -> None:
        """Fetch the options ladder and buy a call or put at this level."""
        # Fetch ladder (cached per tick)
        contracts_list = self._fetch_ladder(bar_dict)
        if not contracts_list:
            self.funnel["entries_skipped_no_contract"] += 1
            return

        # Select contract
        selected = self._select_contract(level, contracts_list, group, bar_dict)
        if selected is None:
            self.funnel["entries_skipped_no_contract"] += 1
            self.logger.debug(
                f"  No suitable contract [{group.group_id}] level {level_idx}"
            )
            return

        premium = selected.ask
        if premium <= 0:
            self.funnel["entries_skipped_no_contract"] += 1
            return

        # Check bid-ask spread
        mid = (selected.bid + selected.ask) / 2
        if mid > 0 and (selected.ask - selected.bid) / mid > 0.20:
            self.funnel["entries_skipped_wide_spread"] += 1
            self.logger.debug(
                f"  Wide spread [{group.group_id}] level {level_idx}:"
                f" bid={selected.bid:.2f} ask={selected.ask:.2f}"
            )
            return

        # Budget check
        equity = self.playground.account.equity
        group_budget = self.max_premium_pct * equity
        cost_per_contract = premium * 100  # contract_size = 100
        budget_remaining = group_budget - group.total_premium_spent

        target_contracts = level.shares  # shares field = allocated contracts
        max_affordable = int(budget_remaining / cost_per_contract) if cost_per_contract > 0 else 0

        if max_affordable <= 0:
            self.funnel["entries_skipped_budget"] += 1
            self.logger.debug(
                f"  Budget exhausted [{group.group_id}] level {level_idx}:"
                f" remaining=${budget_remaining:.2f}, cost/contract=${cost_per_contract:.2f}"
            )
            return

        contracts = min(target_contracts, max_affordable)
        total_premium = contracts * cost_per_contract

        # Compute DTE for horizon selection
        current_ts = bar_dict.get("datetime")
        dte_days = 14  # default
        if current_ts and selected.expiration_date:
            try:
                exp_dt = datetime.fromisoformat(
                    selected.expiration_date.replace("Z", "+00:00")
                )
                if hasattr(current_ts, "date"):
                    dte_days = max(1, (exp_dt.date() - current_ts.date()).days)
                else:
                    dte_days = 14
            except (ValueError, TypeError):
                dte_days = 14

        # Compute expected profit via distribution integration
        expected_profit = self._compute_expected_profit(
            group, level, selected, contracts, dte_days=dte_days,
        )

        # Compute time-bounded p_revert for logging alongside the unbounded value
        p_revert_bounded = self._compute_time_bounded_p_revert(group)

        # Option 3: skip entries with negative or insufficient expected profit
        if expected_profit < self.min_expected_profit:
            self.funnel["entries_skipped_expected_profit"] += 1
            self.logger.debug(
                f"  Skipping entry [{group.group_id}] level {level_idx}:"
                f" expected_profit=${expected_profit:.2f}"
                f" < min=${self.min_expected_profit:.2f}"
                f" (p_revert_unbounded={level.p_revert:.4f},"
                f" p_revert_bounded={p_revert_bounded:.4f})"
            )
            return

        option_type = "call" if group.direction == "bullish" else "put"
        stock_price = bar_dict.get("close", 0.0)

        attributes = {
            "group_id": group.group_id,
            "htf_signal": group.htf_signal_key,
            "signal_price": str(group.htf_signal_price),
            "level_index": str(level_idx),
            "action": "entry",
            "direction": group.direction,
            "option_type": option_type,
            "premium_per_contract": f"{premium:.2f}",
            "strike": f"{selected.strike:.2f}",
            "expiration": selected.expiration_date,
            "p_revert_unbounded": f"{level.p_revert:.4f}",
            "p_revert_bounded": f"{p_revert_bounded:.4f}",
            "sigma_distance": f"{level.sigma_distance:.2f}",
            "strike_strategy": self.strike_strategy,
            "expected_profit": f"{expected_profit:.2f}",
            "ev_per_contract": f"{expected_profit / contracts:.2f}" if contracts > 0 else "0.00",
            "dte_days": str(dte_days),
            "model_name": group.model_name,
        }

        try:
            self.playground.place_order(
                selected.symbol,
                contracts,
                OrderSide.BUY_TO_OPEN,
                "option",
                premium,
                attributes=attributes,
            )

            entry = OptionsContractEntry(
                contract_symbol=selected.symbol,
                option_type=option_type,
                strike=selected.strike,
                contracts=contracts,
                premium_per_contract=premium,
                total_premium=total_premium,
                level_index=level_idx,
                sigma_distance=level.sigma_distance,
                p_revert=level.p_revert,
                entry_stock_price=stock_price,
                expiration_date=selected.expiration_date,
            )
            group.entries[level_idx] = entry
            group.total_premium_spent += total_premium
            if group.status == "pending":
                group.status = "active"
            self.funnel["entries_placed"] += 1
            ev_pc = expected_profit / contracts if contracts > 0 else 0.0
            self.logger.info(
                f"  Entry [{group.group_id}]: {contracts}x {option_type}"
                f" @ strike ${selected.strike:.2f}"
                f" premium ${premium:.2f} (σ={level.sigma_distance:.1f})"
                f" p_revert_bounded={p_revert_bounded:.4f}"
                f" EV=${expected_profit:.2f} (${ev_pc:.2f}/contract)"
                f" DTE={dte_days}"
            )
        except Exception as e:
            self.logger.warning(
                f"  Entry failed [{group.group_id}] level {level_idx}: {e}"
            )

    # ------------------------------------------------------------------ #
    # Exit checks
    # ------------------------------------------------------------------ #

    def _check_exits(self, group: OptionsTradeGroup, bar_dict: dict) -> None:
        """Check exit conditions for an active group.

        Priority order (Option 6 — option-price-based exit is primary):
        1. Profit target: option value >= (1 + profit_target_pct) * premium
        2. Reversion complete: stock reverted to signal price — exit at
           current option value (regardless of profit)
        3. Time decay: DTE <= threshold → force close
        """
        if group.status != "active":
            return

        high = bar_dict.get("high", 0.0)
        low = bar_dict.get("low", float("inf"))

        # Get current simulation timestamp for DTE checks
        current_ts = bar_dict.get("datetime")
        if isinstance(current_ts, str):
            from dateutil.parser import parse as parse_dt
            current_ts = parse_dt(current_ts)

        # 1. PRIMARY: option-price-based profit target (per entry)
        for level_idx, entry in list(group.entries.items()):
            if level_idx in group.exited_entries:
                continue
            if entry.candles_held < self.min_hold_candles:
                continue

            position = self.playground.account.get_position(entry.contract_symbol)
            if position and position.current_price > 0:
                current_value = position.current_price * entry.contracts * 100
                if current_value >= (1 + self.profit_target_pct) * entry.total_premium:
                    self._place_exit(group, entry, "exit_profit")
                    continue

        if self._is_group_fully_exited(group):
            group.status = "closed"
            self.funnel["groups_closed"] += 1
            self.logger.info(f"  Group {group.group_id} closed (profit target)")
            return

        # 2. SECONDARY: reversion complete — exit remaining entries at
        #    current option value (the stock has reverted, so take what we have)
        reversion_complete = False
        if group.direction == "bullish" and high >= group.htf_signal_price:
            reversion_complete = True
        elif group.direction == "bearish" and low <= group.htf_signal_price:
            reversion_complete = True

        if reversion_complete:
            for level_idx, entry in list(group.entries.items()):
                if level_idx in group.exited_entries:
                    continue
                if entry.candles_held < self.min_hold_candles:
                    continue
                self._place_exit(group, entry, "exit_reversion")
            if self._is_group_fully_exited(group):
                group.status = "closed"
                self.funnel["groups_closed"] += 1
                self.logger.info(f"  Group {group.group_id} closed (reversion)")
            return

        # 3. TERTIARY: time decay — force exit per entry
        for level_idx, entry in list(group.entries.items()):
            if level_idx in group.exited_entries:
                continue
            if entry.candles_held < self.min_hold_candles:
                continue

            if current_ts and entry.expiration_date:
                try:
                    exp_dt = datetime.fromisoformat(
                        entry.expiration_date.replace("Z", "+00:00")
                    )
                    if current_ts.tzinfo is None:
                        from zoneinfo import ZoneInfo
                        current_ts = current_ts.replace(tzinfo=ZoneInfo("America/New_York"))
                    dte = (exp_dt.date() - current_ts.date()).days
                    if dte <= self.time_decay_exit_dte:
                        self._place_exit(group, entry, "exit_time_decay")
                        continue
                except (ValueError, TypeError):
                    pass

        if self._is_group_fully_exited(group):
            group.status = "closed"
            self.funnel["groups_closed"] += 1

    def _place_exit(
        self, group: OptionsTradeGroup, entry: OptionsContractEntry, action: str,
    ) -> None:
        """Sell-to-close an options position."""
        # Check we still have a position
        position = self.playground.account.get_position(entry.contract_symbol)
        qty = int(position.quantity) if position else 0
        if qty <= 0:
            group.exited_entries.add(entry.level_index)
            return

        contracts_to_sell = min(entry.contracts, qty)

        attributes = {
            "group_id": group.group_id,
            "action": action,
            "level_index": str(entry.level_index),
        }

        try:
            self.playground.place_order(
                entry.contract_symbol,
                contracts_to_sell,
                OrderSide.SELL_TO_CLOSE,
                "option",
                attributes=attributes,
            )
            group.exited_entries.add(entry.level_index)
            self.funnel[f"exits_{action.replace('exit_', '')}"] += 1
            self.logger.info(
                f"  {action} [{group.group_id}]: {contracts_to_sell}x"
                f" {entry.contract_symbol}"
            )
        except Exception as e:
            self.logger.warning(
                f"  Exit failed [{group.group_id}] {entry.contract_symbol}: {e}"
            )

    def _is_group_fully_exited(self, group: OptionsTradeGroup) -> bool:
        """Check if all entries in a group have been exited."""
        return len(group.exited_entries) >= len(group.entries) and len(group.entries) > 0

    # ------------------------------------------------------------------ #
    # Options ladder and contract selection
    # ------------------------------------------------------------------ #

    def _fetch_ladder(self, bar_dict: dict):
        """Fetch options ladder, cached per tick."""
        tick_ts = bar_dict.get("datetime")
        if self._cached_ladder is not None and self._cached_ladder_tick_ts == tick_ts:
            return self._cached_ladder

        # Compute expiration days
        current_ts = bar_dict.get("datetime")
        if isinstance(current_ts, str):
            from dateutil.parser import parse as parse_dt
            current_ts = parse_dt(current_ts)

        exp_days = self._compute_expiration_days(current_ts)
        if not exp_days:
            self.funnel["ladder_empty"] += 1
            return None

        request = GetOptionsLadderRequest(
            playground_id=self.playground.id,
            stock_symbol=self.symbol,
            max_no_of_strikes=15,
            min_distance_between_strikes=1.0,
            expiration_in_days=exp_days,
            max_tick_age_in_minutes=1440,
        )

        try:
            response = self.playground.fetch_ladder(request)
            self.funnel["ladder_fetches"] += 1
        except Exception as e:
            self.logger.warning(f"  Ladder fetch failed: {e}")
            self.funnel["ladder_empty"] += 1
            return None

        if not response or not response.contracts:
            self.funnel["ladder_empty"] += 1
            self._cached_ladder = None
            self._cached_ladder_tick_ts = tick_ts
            return None

        contracts = list(response.contracts)
        self._cached_ladder = contracts
        self._cached_ladder_tick_ts = tick_ts
        return contracts

    def _compute_expiration_days(self, current_ts) -> list:
        """Compute candidate expiration_in_days values."""
        if current_ts is None:
            return [self.target_dte]

        # Find the next Friday from current_ts at target_dte
        # Return a few candidate DTEs within [min_dte, max_dte]
        candidates = []
        for dte in range(self.min_dte, self.max_dte + 1):
            future = current_ts + timedelta(days=dte)
            if future.weekday() == 4:  # Friday
                candidates.append(dte)

        if not candidates:
            # Fallback: use target_dte directly
            return [self.target_dte]

        # Return the closest Friday to target_dte and the next one
        closest = min(candidates, key=lambda d: abs(d - self.target_dte))
        result = [closest]
        # Add the next Friday too for more options
        next_friday = closest + 7
        if next_friday <= self.max_dte:
            result.append(next_friday)
        return result

    def _select_contract(self, level, contracts_list, group, bar_dict):
        """Select the best option contract for a deviation level."""
        option_type = "call" if group.direction == "bullish" else "put"
        stock_price = bar_dict.get("close", 0.0)

        # Filter by type
        candidates = [c for c in contracts_list if c.type == option_type]
        if not candidates:
            return None

        # Filter by ask > 0
        candidates = [c for c in candidates if c.ask > 0]
        if not candidates:
            return None

        # Compute target strike based on strategy
        target_strike = self._compute_target_strike(
            level, group, stock_price,
        )

        # Find closest strike to target
        best = min(candidates, key=lambda c: abs(c.strike - target_strike))
        return best

    def _compute_target_strike(self, level, group, stock_price: float) -> float:
        """Compute the target strike price based on strike_strategy.

        The ``"model"`` strategy anchors strikes ITM relative to the signal
        price so that a successful reversion produces meaningful intrinsic
        value that exceeds the premium paid.
        """
        signal_price = group.htf_signal_price
        level_price = level.price
        sigma = level.sigma_distance

        if self.strike_strategy == "model":
            # Anchor strike ITM so intrinsic at signal > premium.
            # Deeper dips (higher sigma) → deeper ITM for higher delta.
            distance = abs(signal_price - level_price)
            if group.direction == "bullish":
                # Call: strike below level_price → ITM at entry, large
                # intrinsic at signal.  intrinsic_at_signal = signal - strike
                # = signal - (level - k*distance) = (1+k)*distance
                if sigma < 1.5:
                    return level_price - 0.5 * distance
                elif sigma < 2.5:
                    return level_price - 1.0 * distance
                else:
                    return level_price - 1.5 * distance
            else:
                # Put: strike above level_price → ITM at entry, large
                # intrinsic at signal.  intrinsic_at_signal = strike - signal
                # = (level + k*distance) - signal = (1+k)*distance
                if sigma < 1.5:
                    return level_price + 0.5 * distance
                elif sigma < 2.5:
                    return level_price + 1.0 * distance
                else:
                    return level_price + 1.5 * distance

        elif self.strike_strategy == "atm":
            return stock_price

        elif self.strike_strategy == "otm":
            if group.direction == "bullish":
                # OTM call: strike above stock price
                return stock_price + 0.5 * (signal_price - stock_price)
            else:
                # OTM put: strike below stock price
                return stock_price - 0.5 * (stock_price - signal_price)

        # Default: at level price
        return level_price

    # ------------------------------------------------------------------ #
    # Time-bounded reversion probability
    # ------------------------------------------------------------------ #

    @staticmethod
    def _horizon_sort_key(name: str) -> int:
        """Return a sort key (approximate bar count) for a horizon name.

        Handles standard names like '1h', '4h', '1d', '2d', '1w', '2w'.
        Falls back to len(forward_returns) comparison if name is unknown.
        """
        _KNOWN = {
            "1h": 4, "4h": 16, "1d": 26, "2d": 52,
            "1w": 130, "2w": 260, "1m": 520,
        }
        return _KNOWN.get(name, 0)

    def _compute_time_bounded_p_revert(
        self, group: OptionsTradeGroup,
    ) -> float:
        """
        Compute time-bounded reversion probability using the longest
        available PDF horizon.

        Unlike the unbounded p_revert (which measures P(return >= -dip)),
        this measures P(return >= 0) at the longest horizon — i.e., did
        the stock actually revert to signal price within that window?
        """
        best_horizon = None
        best_bars = 0
        for name, stats in group.horizons.items():
            bars = self._horizon_sort_key(name)
            if bars > best_bars and stats.forward_returns:
                best_bars = bars
                best_horizon = stats

        if best_horizon is None or not best_horizon.forward_returns:
            return 0.5  # neutral fallback

        returns = np.array(best_horizon.forward_returns)
        if group.direction == "bearish":
            # For bearish signals, reversion means price fell back down
            # (return went negative from signal), so check return <= 0
            return float(np.mean(returns <= 0))
        return float(np.mean(returns >= 0))

    # ------------------------------------------------------------------ #
    # Horizon selection
    # ------------------------------------------------------------------ #

    # Approximate calendar-day equivalents for each horizon name.
    _HORIZON_DAYS: Dict[str, float] = {
        "1h": 1 / 6.5,   # ~0.15 days
        "4h": 4 / 6.5,   # ~0.62 days
        "1d": 1.0,
        "2d": 2.0,
        "1w": 5.0,        # 5 trading days
        "2w": 10.0,       # 10 trading days
        "1m": 21.0,       # ~21 trading days
    }

    def _select_horizon_for_dte(
        self, group: OptionsTradeGroup, dte_days: int,
    ) -> Optional[HorizonStats]:
        """Select the PDF horizon best matching the option's DTE.

        Picks the horizon with the most calendar days that doesn't exceed
        the option's DTE. Falls back to the longest available horizon if
        all are shorter than the DTE, or if dte_days <= 0.
        """
        best = None
        best_days = -1.0
        longest = None
        longest_days = -1.0

        for name, stats in group.horizons.items():
            if not stats.forward_returns:
                continue
            h_days = self._HORIZON_DAYS.get(name, 0.0)
            # Track the longest horizon as fallback
            if h_days > longest_days:
                longest_days = h_days
                longest = stats
            # Best match: largest horizon that fits within DTE
            if h_days <= dte_days and h_days > best_days:
                best_days = h_days
                best = stats

        return best if best is not None else longest

    # ------------------------------------------------------------------ #
    # Expected profit
    # ------------------------------------------------------------------ #

    def _compute_expected_profit(
        self, group, level, contract, contracts: int, dte_days: int = 14,
    ) -> float:
        """Compute expected profit by integrating over the forward return
        distribution.

        For each historical forward return r_i at the DTE-matched horizon,
        maps to a stock exit price and computes option intrinsic value.
        The expected profit is the average intrinsic across all returns
        minus the total premium paid.

        This captures partial reversion, overshoot, and the full shape
        of the return distribution — not just binary revert/don't-revert.
        """
        signal_price = group.htf_signal_price
        strike = contract.strike
        total_premium = contracts * contract.ask * 100

        # Select horizon matched to option DTE
        horizon = self._select_horizon_for_dte(group, dte_days)
        if horizon is None or not horizon.forward_returns:
            # Fallback: no horizon data, use conservative 0
            return -total_premium

        returns = np.array(horizon.forward_returns)

        # Map each return to exit stock price
        exit_prices = signal_price * (1.0 + returns)

        # Compute intrinsic value at each exit price
        if group.direction == "bullish":
            intrinsics = np.maximum(0.0, exit_prices - strike)
        else:
            intrinsics = np.maximum(0.0, strike - exit_prices)

        expected_value = float(np.mean(intrinsics)) * 100 * contracts
        return expected_value - total_premium

    # ------------------------------------------------------------------ #
    # End-of-simulation cleanup
    # ------------------------------------------------------------------ #

    def close_all_active_groups(self) -> None:
        """Close all remaining active groups (end of sim)."""
        for group in self.trade_groups:
            if group.status not in ("pending", "active"):
                continue
            for level_idx, entry in group.entries.items():
                if level_idx in group.exited_entries:
                    continue
                self._place_exit(group, entry, "end_of_sim_close")
            group.status = "closed"

    # ------------------------------------------------------------------ #
    # Summary
    # ------------------------------------------------------------------ #

    def log_summary(self) -> None:
        """Log funnel summary and group statistics."""
        self.logger.info("=" * 60)
        self.logger.info("OPTIONS MEAN-REVERSION FUNNEL SUMMARY")
        self.logger.info("=" * 60)
        for key, val in self.funnel.items():
            self.logger.info(f"  {key:35s}: {val}")
        self.logger.info("-" * 60)

        active = sum(1 for g in self.trade_groups if g.status == "active")
        closed = sum(1 for g in self.trade_groups if g.status == "closed")
        pending = sum(1 for g in self.trade_groups if g.status == "pending")
        bullish = sum(1 for g in self.trade_groups if g.direction == "bullish")
        bearish = sum(1 for g in self.trade_groups if g.direction == "bearish")

        total_premium = sum(g.total_premium_spent for g in self.trade_groups)

        self.logger.info(f"  Groups total:     {len(self.trade_groups)}")
        self.logger.info(f"  Groups bullish:   {bullish}")
        self.logger.info(f"  Groups bearish:   {bearish}")
        self.logger.info(f"  Groups active:    {active}")
        self.logger.info(f"  Groups closed:    {closed}")
        self.logger.info(f"  Groups pending:   {pending}")
        self.logger.info(f"  Total premium:    ${total_premium:,.2f}")
        self.logger.info("=" * 60)

    # ------------------------------------------------------------------ #
    # BaseStrategy interface methods
    # ------------------------------------------------------------------ #

    def on_tick(self, tick_deltas) -> None:
        """Process tick deltas by extracting candles and delegating to process_candles."""
        for td in tick_deltas:
            new_candles = td.new_candles if hasattr(td, 'new_candles') else []
            self.process_candles(new_candles)

    def get_next_tick_seconds(self) -> int:
        """Smart tick: LTF when active trades exist, HTF when idle (D-03)."""
        has_active = any(
            g.status in ('pending', 'active') for g in self.trade_groups
        )
        return self.playground.ltf_seconds if has_active else self.playground.htf_seconds

    def should_fetch_account(self) -> bool:
        """Fetch account when active trades exist."""
        return any(g.status in ('pending', 'active') for g in self.trade_groups)

    def on_complete(self) -> None:
        """End-of-sim: close all active groups and log summary."""
        self.close_all_active_groups()
        self.log_summary()


# ------------------------------------------------------------------ #
# Bar conversion helper
# ------------------------------------------------------------------ #

def _bar_to_dict(bar) -> dict:
    """Convert a protobuf Bar to a dict for signal detection."""
    return {
        "open": _get(bar, "open"),
        "high": _get(bar, "high"),
        "low": _get(bar, "low"),
        "close": _get(bar, "close"),
        "datetime": _get_dt(bar),
        "superD_50_3": _get(bar, "superD_50_3"),
        "stochrsi_k_14_14_3_3": _get(bar, "stochrsi_k_14_14_3_3"),
        "stochrsi_cross_above_20": _get(bar, "stochrsi_cross_above_20"),
        "stochrsi_cross_below_80": _get(bar, "stochrsi_cross_below_80"),
        "cdl_hammer": _get(bar, "cdl_hammer"),
        "cdl_doji_10_0_1": _get(bar, "cdl_doji_10_0_1"),
        "sma_50": _get(bar, "sma_50"),
        "sma_100": _get(bar, "sma_100"),
        "sma_200": _get(bar, "sma_200"),
    }


# ------------------------------------------------------------------ #
# Runner function
# ------------------------------------------------------------------ #

def run_options_mean_reversion(
    playground: BacktesterPlaygroundClient,
    symbol: str,
    logger,
    pdf: PDFDocument,
    max_premium_pct: float = 0.02,
    stop_percentile: float = 0.95,
    total_contracts_per_group: int = 0,
    htf_horizon: str = "1h",
    target_dte: int = 14,
    min_dte: int = 5,
    max_dte: int = 30,
    profit_target_pct: float = 0.50,
    time_decay_exit_dte: int = 3,
    strike_strategy: str = "model",
    min_hold_candles: int = 6,
    tail_threshold: float = 0.005,
    min_expected_profit: float = 50.0,
    long_only: bool = False,
    on_tick=None,
) -> OptionsMeanReversionStrategy:
    """
    Main loop for the Options Mean-Reversion Strategy.

    Parameters
    ----------
    on_tick : callable, optional
        Callback invoked after each tick batch with ``(strategy, tick_deltas)``.
        Can be used for periodic PDF retraining.

    Returns the strategy instance.
    """
    strategy = OptionsMeanReversionStrategy(
        playground, symbol, logger,
        pdf=pdf,
        max_premium_pct=max_premium_pct,
        stop_percentile=stop_percentile,
        total_contracts_per_group=total_contracts_per_group,
        htf_horizon=htf_horizon,
        target_dte=target_dte,
        min_dte=min_dte,
        max_dte=max_dte,
        profit_target_pct=profit_target_pct,
        time_decay_exit_dte=time_decay_exit_dte,
        strike_strategy=strike_strategy,
        min_hold_candles=min_hold_candles,
        tail_threshold=tail_threshold,
        min_expected_profit=min_expected_profit,
        long_only=long_only,
    )

    max_iterations = 500_000
    iteration = 0

    while not playground.is_backtest_complete():
        iteration += 1
        if iteration > max_iterations:
            logger.warning(f"Max iterations ({max_iterations}) reached, stopping")
            break

        tick_deltas = playground.flush_new_state_buffer()
        for tick_delta in tick_deltas:
            new_candles = (
                tick_delta.new_candles
                if hasattr(tick_delta, "new_candles")
                else []
            )
            strategy.process_candles(new_candles)

        if on_tick is not None:
            on_tick(strategy, tick_deltas)

        # When idle (no open/pending groups), tick by HTF period instead of LTF
        # to skip ~12x fewer RPCs. The server returns all completed candles in
        # the interval, so HTF signals and LTF entries are still processed.
        has_active = any(
            g.status in ("pending", "active") for g in strategy.trade_groups
        )
        tick_seconds = playground.ltf_seconds if has_active else playground.htf_seconds
        playground.tick(tick_seconds, fetch_account=has_active)

    # End-of-sim cleanup
    strategy.close_all_active_groups()
    strategy.log_summary()

    return strategy
