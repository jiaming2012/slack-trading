"""
PDF Mean-Reversion Strategy.

Multi-timeframe equity strategy:
- HTF (1-hour, configurable): detects bullish compound signals via PDF
- LTF (5-min): buys dips at σ-deviation bands when HTF is bullish
- Scales in at deeper levels where P(revert) is higher
- Partial exits as price returns toward HTF signal price
- Stops out entire group on HTF 95th percentile adverse close

Long-only, single-symbol, accepts overnight risk.
"""

from __future__ import annotations

import time
import uuid
from dataclasses import dataclass, field
from datetime import datetime
from typing import Dict, List, Optional, Set, Tuple
from zoneinfo import ZoneInfo

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
from lib.partial_exit_manager import ExitPlan, check_exits, compute_exit_plan, _tier_fraction
from lib.pdf_builder import detect_atomic_signals_on_bar, _get, _get_dt
from lib.pdf_types import PDFDocument, SignalPDF
from engine.types import OrderSide
from strategies.base_strategy import BaseStrategy


# ------------------------------------------------------------------ #
# Data structures
# ------------------------------------------------------------------ #

@dataclass
class TradeGroup:
    """One group of trades spawned by a single HTF signal."""

    group_id: str
    htf_signal_key: str
    htf_signal_price: float
    htf_signal_timestamp: datetime
    deviation_plan: DeviationPlan
    exit_plan: Optional[ExitPlan] = None
    filled_levels: Dict[int, int] = field(default_factory=dict)
    failed_levels: Set[int] = field(default_factory=set)
    triggered_exits: Set[tuple] = field(default_factory=set)  # (source_level_index, tier_index)
    status: str = "pending"  # pending, active, stopped_out, closed
    stop_price: float = 0.0
    model_name: str = "empirical"
    forward_returns: List[float] = field(default_factory=list)


# ------------------------------------------------------------------ #
# Strategy class
# ------------------------------------------------------------------ #

class MeanReversionStrategy(BaseStrategy):
    """
    PDF-guided mean-reversion strategy.

    Parameters
    ----------
    playground : BacktesterPlaygroundClient
        The backtester playground client.
    symbol : str
        Underlying stock symbol.
    logger : loguru.Logger
        Logger instance.
    pdf : PDFDocument
        Pre-built PDF with signal distributions.
    max_loss_pct : float
        Max loss per group as fraction of equity (default 0.02 = 2%).
    stop_percentile : float
        Percentile for HTF stop (default 0.95).
    total_shares_per_group : int
        Total shares to distribute across levels (0 = auto-size from balance/price).
    num_exit_tiers : int
        Number of partial exit tiers per level (default 3).
    htf_horizon : str
        Which PDF horizon to use for deviation levels (default "1h").
    """

    def __init__(
        self,
        playground: BacktesterPlaygroundClient,
        symbol: str,
        logger=None,
        pdf: PDFDocument = None,
        max_loss_pct: float = 0.02,
        stop_percentile: float = 0.95,
        total_shares_per_group: int = 0,
        num_exit_tiers: int = 3,
        htf_horizon: str = "1h",
        tier_spacing: str = "even",
        stop_widen_on_exit: float = 1.0,
        min_expected_profit: float = 0.0,
        ev_model: str = "distribution",
    ):
        super().__init__(playground, symbol, logger or _default_logger)
        self.pdf = pdf
        self.max_loss_pct = max_loss_pct
        self.stop_percentile = stop_percentile

        # Auto-size shares from balance / current price when not specified
        if total_shares_per_group <= 0:
            ltf_bar = playground.current_candles.get(symbol, {}).get(playground.ltf_seconds)
            current_price = ltf_bar.close if ltf_bar else 0
            if current_price > 0:
                total_shares_per_group = max(1, int(playground.account.balance / current_price))
            else:
                total_shares_per_group = 1
            self.logger.info(
                f"Auto-sized shares/group: {total_shares_per_group}"
                f" (balance ${playground.account.balance:,.2f}"
                f" / price ${current_price:.2f})"
            )

        self.total_shares_per_group = total_shares_per_group
        self.num_exit_tiers = num_exit_tiers
        self.htf_horizon = htf_horizon
        self.tier_spacing = tier_spacing
        self.stop_widen_on_exit = stop_widen_on_exit
        self.min_expected_profit = min_expected_profit
        self.ev_model = ev_model

        # State
        self.trade_groups: List[TradeGroup] = []
        self._prev_htf_bar: Optional[dict] = None
        self._prev_ltf_bar: Optional[dict] = None
        self._htf_signal_dedup: Set[str] = set()  # (timestamp_iso, signal_key)
        self._margin_used_this_tick: float = 0.0  # tracks margin consumed within a tick
        self._sells_placed_this_tick: float = 0.0  # tracks sell shares placed within a tick (not yet settled)

        # Funnel counters
        self.funnel = {
            "htf_bars": 0,
            "ltf_bars": 0,
            "signals_detected": 0,
            "groups_created": 0,
            "groups_skipped_budget": 0,
            "groups_skipped_empty_plan": 0,
            "groups_skipped_dedup": 0,
            "entries_placed": 0,
            "exits_placed": 0,
            "stop_outs": 0,
            "groups_closed": 0,
            "groups_truncated": 0,
            "entries_skipped_low_ev": 0,
        }

    def _effective_server_position(self) -> float:
        """Server position minus sells already placed this tick but not yet settled."""
        return self.playground.account.get_quantity(self.symbol) - self._sells_placed_this_tick

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
        """
        Process new candles from a tick delta.

        HTF candles: detect signals, create groups, evaluate stops.
        LTF candles: check entries and exits.
        """
        self._margin_used_this_tick = 0.0
        self._sells_placed_this_tick = 0.0
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
            # Build compound signal key
            key = "|".join(sorted(signals))

            # Look up in PDF
            pdf_entry = self.pdf.get_signal(key)
            if pdf_entry and pdf_entry.sufficient_samples:
                horizon = pdf_entry.horizons.get(self.htf_horizon)
                if horizon and horizon.mean > 0:
                    self._try_create_group(
                        key, bar_dict, pdf_entry,
                    )

        # 2. Evaluate stops on all active groups
        htf_close = bar_dict.get("close", 0.0)
        self._evaluate_stops(htf_close)

    def _process_ltf_candle(self, bar_dict: dict) -> None:
        """Handle a completed LTF candle."""
        self.funnel["ltf_bars"] += 1
        self._prev_ltf_bar = bar_dict

        low = bar_dict.get("low", float("inf"))
        high = bar_dict.get("high", 0.0)

        # Track which groups got new fills this candle so we skip
        # exit checks for them — the position hasn't settled yet.
        groups_with_new_fills: set = set()

        # Check entries for active/pending groups
        for group in self.trade_groups:
            if group.status not in ("pending", "active"):
                continue
            fills_before = len(group.filled_levels)
            self._check_entries(group, low)
            if len(group.filled_levels) > fills_before:
                groups_with_new_fills.add(id(group))

        # Check exits (skip groups that just got new fills this candle)
        for group in self.trade_groups:
            if group.status != "active":
                continue
            if id(group) in groups_with_new_fills:
                continue
            self._check_exits(group, high)

    # ------------------------------------------------------------------ #
    # Group creation
    # ------------------------------------------------------------------ #

    def _try_create_group(
        self, signal_key: str, bar_dict: dict, pdf_entry: SignalPDF,
    ) -> None:
        """Attempt to create a new trade group from an HTF signal."""
        self.funnel["signals_detected"] += 1

        timestamp = bar_dict.get("datetime", datetime.now())
        if isinstance(timestamp, str):
            from dateutil.parser import parse as parse_dt
            timestamp = parse_dt(timestamp)

        # Dedup: same signal from same HTF bar
        dedup_key = f"{timestamp.isoformat()}|{signal_key}"
        if dedup_key in self._htf_signal_dedup:
            self.funnel["groups_skipped_dedup"] += 1
            return
        self._htf_signal_dedup.add(dedup_key)

        signal_price = bar_dict.get("close", 0.0)
        horizon = pdf_entry.horizons.get(self.htf_horizon)
        if not horizon:
            return

        # Compute max loss budget
        equity = self.playground.account.equity
        max_loss_budget = self.max_loss_pct * equity

        # Build deviation plan
        plan = compute_deviation_levels(
            signal_price=signal_price,
            stddev=horizon.stddev,
            forward_returns=horizon.forward_returns,
            stop_percentile=self.stop_percentile,
            max_loss_budget=max_loss_budget,
            total_shares=self.total_shares_per_group,
        )

        if not plan.levels:
            self.funnel["groups_skipped_empty_plan"] += 1
            self.logger.debug(
                f"Empty deviation plan for signal {signal_key} at ${signal_price:.2f}"
            )
            return

        if plan.was_truncated:
            self.funnel["groups_truncated"] += 1
            self.logger.info(
                f"Budget truncation: {plan.original_level_count} → {len(plan.levels)} levels"
                f" (max_loss=${plan.max_potential_loss:.2f}, budget=${max_loss_budget:.2f})"
            )

        group = TradeGroup(
            group_id=str(uuid.uuid4())[:8],
            htf_signal_key=signal_key,
            htf_signal_price=signal_price,
            htf_signal_timestamp=timestamp,
            deviation_plan=plan,
            stop_price=plan.stop_price,
            forward_returns=list(horizon.forward_returns) if horizon.forward_returns else [],
        )

        self.trade_groups.append(group)
        self.funnel["groups_created"] += 1
        self.logger.info(
            f"New trade group {group.group_id}: signal={signal_key}"
            f" price=${signal_price:.2f} levels={len(plan.levels)}"
            f" stop=${plan.stop_price:.2f}"
        )

    # ------------------------------------------------------------------ #
    # Entry checks
    # ------------------------------------------------------------------ #

    def _check_entries(self, group: TradeGroup, candle_low: float) -> None:
        """Check if any unfilled levels are breached by candle low."""
        for idx, level in enumerate(group.deviation_plan.levels):
            if idx in group.filled_levels or idx in group.failed_levels:
                continue
            if candle_low <= level.price:
                self._place_entry(group, idx, level)

    def _place_entry(self, group: TradeGroup, level_idx: int, level) -> None:
        """Place a buy order for an entry level.

        Caps shares based on available margin (50% initial margin requirement)
        to prevent silent server-side rejections during tick processing.
        """
        # Cap shares to available margin (50% initial margin requirement).
        # Account for margin already committed by earlier entries this tick
        # that haven't been filled yet (account.free_margin is stale within a tick).
        shares = level.shares
        free_margin = self.playground.account.free_margin - self._margin_used_this_tick
        initial_margin_per_share = level.price * 0.5
        if initial_margin_per_share > 0:
            max_affordable = int(free_margin * 0.95 / initial_margin_per_share)
            if max_affordable <= 0:
                group.failed_levels.add(level_idx)
                self.logger.warning(
                    f"  Entry skipped [{group.group_id}] level {level_idx}:"
                    f" insufficient margin (free=${free_margin:.0f},"
                    f" need=${initial_margin_per_share:.0f}/share)"
                )
                return
            if max_affordable < shares:
                self.logger.info(
                    f"  Margin cap [{group.group_id}] level {level_idx}:"
                    f" {shares} → {max_affordable} shares"
                    f" (free_margin=${free_margin:.0f})"
                )
                shares = max_affordable

        # Compute expected profit for this level.
        #
        # Model the actual exit mechanics: on reversion, shares are sold
        # across partial exit tiers (not all at signal_price).  The weighted
        # average exit price across tiers is lower than signal_price.
        # On stop-out, all shares exit at stop_price.
        n_tiers = self.num_exit_tiers
        if n_tiers > 0 and group.htf_signal_price > level.price:
            distance = group.htf_signal_price - level.price
            # Mirror the tier spacing from compute_exit_plan:
            # last tier at signal_price, earlier tiers at fraction * distance
            tier_exits = []
            for ti in range(n_tiers):
                if ti == n_tiers - 1:
                    tier_exits.append(group.htf_signal_price)
                else:
                    fraction = _tier_fraction(ti, n_tiers, self.tier_spacing)
                    tier_exits.append(level.price + fraction * distance)
            avg_exit_on_revert = sum(tier_exits) / n_tiers
        else:
            avg_exit_on_revert = group.htf_signal_price

        expected_exit = (
            avg_exit_on_revert * level.p_revert
            + group.stop_price * (1 - level.p_revert)
        )
        binary_ev = shares * (expected_exit - level.price)
        distribution_ev = self._compute_distribution_ev(group, level, shares)

        if self.ev_model == "distribution":
            expected_profit = distribution_ev
            alt_label, alt_ev = "expected_profit_binary", binary_ev
        else:
            expected_profit = binary_ev
            alt_label, alt_ev = "expected_profit_distribution", distribution_ev

        if self.min_expected_profit > 0 and expected_profit < self.min_expected_profit:
            group.failed_levels.add(level_idx)
            self.funnel["entries_skipped_low_ev"] += 1
            self.logger.info(
                f"  Entry skipped [{group.group_id}] level {level_idx}:"
                f" EV=${expected_profit:.2f} < min=${self.min_expected_profit:.2f}"
            )
            return

        attributes = {
            "group_id": group.group_id,
            "htf_signal": group.htf_signal_key,
            "signal_price": str(group.htf_signal_price),
            "level_index": str(level_idx),
            "action": "entry",
            "expected_profit": f"{expected_profit:.2f}",
            alt_label: f"{alt_ev:.2f}",
            "ev_model": self.ev_model,
            "p_revert": f"{level.p_revert:.4f}",
            "model_name": group.model_name,
            "sigma_distance": f"{level.sigma_distance:.2f}",
            "stop_price": f"{group.stop_price:.2f}",
        }

        try:
            self.playground.place_order(
                self.symbol,
                shares,
                OrderSide.BUY,
                "equity",
                level.price,
                attributes=attributes,
            )
            group.filled_levels[level_idx] = shares
            if group.status == "pending":
                group.status = "active"
            self.funnel["entries_placed"] += 1
            # Track margin consumed so subsequent entries this tick see reduced availability
            self._margin_used_this_tick += shares * level.price * 0.5
            self.logger.info(
                f"  Entry [{group.group_id}]: {shares} shares"
                f" @ ${level.price:.2f} (σ={level.sigma_distance:.1f})"
            )
            # Recompute exit plan after new fill
            self._recompute_exits(group)
        except Exception as e:
            group.failed_levels.add(level_idx)
            self.logger.warning(
                f"  Entry failed [{group.group_id}] level {level_idx}: {e}"
            )

    def _compute_distribution_ev(
        self, group: TradeGroup, level, shares: int,
    ) -> float:
        """Compute expected profit by iterating over the forward return distribution.

        For each observed forward return, maps to an exit price and computes P&L:
        - Full reversion (exit_price >= signal): uses tiered avg exit
        - Stop-out (exit_price <= stop): uses stop price
        - Partial reversion (between stop and signal): linear interpolation
        """
        if not group.forward_returns:
            return 0.0

        returns = np.array(group.forward_returns)
        exit_prices = group.htf_signal_price * (1.0 + returns)

        # Compute tiered avg exit for full reversion (same logic as binary)
        n_tiers = self.num_exit_tiers
        if n_tiers > 0 and group.htf_signal_price > level.price:
            distance = group.htf_signal_price - level.price
            tier_exits = []
            for ti in range(n_tiers):
                if ti == n_tiers - 1:
                    tier_exits.append(group.htf_signal_price)
                else:
                    fraction = _tier_fraction(ti, n_tiers, self.tier_spacing)
                    tier_exits.append(level.price + fraction * distance)
            avg_exit_on_revert = sum(tier_exits) / n_tiers
        else:
            avg_exit_on_revert = group.htf_signal_price

        pl = np.where(
            exit_prices >= group.htf_signal_price,
            shares * (avg_exit_on_revert - level.price),       # full reversion
            np.where(
                exit_prices <= group.stop_price,
                shares * (group.stop_price - level.price),     # stop-out
                shares * (exit_prices - level.price),          # partial reversion
            ),
        )
        return float(np.mean(pl))

    # ------------------------------------------------------------------ #
    # Exit checks
    # ------------------------------------------------------------------ #

    def _check_exits(self, group: TradeGroup, candle_high: float) -> None:
        """Check if any partial exit tiers are triggered."""
        if group.status != "active" or group.exit_plan is None:
            return

        triggered = check_exits(
            candle_high, group.exit_plan, group.triggered_exits,
        )
        for tier in triggered:
            self._place_exit(group, tier)

    def _place_exit(self, group: TradeGroup, tier) -> None:
        """Place a sell order for a partial exit."""
        tier_id = (tier.source_level_index, tier.tier_index)

        # Use effective position (accounts for sells already placed this tick)
        server_qty = self._effective_server_position()
        shares_to_sell = tier.shares_to_sell

        # Guard: don't sell more than server position allows
        if server_qty <= 0:
            self.logger.warning(
                f"  Exit skipped [{group.group_id}] tier {tier.tier_index}:"
                f" server has no long position (pos={server_qty})"
            )
            # Mark as triggered so we don't retry every candle
            group.triggered_exits.add(tier_id)
            return
        if shares_to_sell > server_qty:
            self.logger.warning(
                f"  Exit clamped [{group.group_id}] tier {tier.tier_index}:"
                f" want to sell {shares_to_sell} but server_pos={server_qty},"
                f" clamping"
            )
            shares_to_sell = server_qty

        attributes = {
            "group_id": group.group_id,
            "action": "partial_exit",
            "exit_tier": str(tier.tier_index),
            "source_level": str(tier.source_level_index),
        }

        try:
            self.playground.place_order(
                self.symbol,
                shares_to_sell,
                OrderSide.SELL,
                "equity",
                tier.exit_price,
                attributes=attributes,
            )
            group.triggered_exits.add(tier_id)
            self._sells_placed_this_tick += shares_to_sell
            self.funnel["exits_placed"] += 1
            self.logger.info(
                f"  Exit [{group.group_id}]: {shares_to_sell} shares"
                f" @ ${tier.exit_price:.2f} (level {tier.source_level_index},"
                f" tier {tier.tier_index})"
            )
            # Check if all exits triggered → close group
            all_tier_ids = {
                (t.source_level_index, t.tier_index)
                for t in group.exit_plan.tiers
            }
            if group.triggered_exits >= all_tier_ids:
                group.status = "closed"
                self.funnel["groups_closed"] += 1
                self.logger.info(f"  Group {group.group_id} fully closed")
        except Exception as e:
            self.logger.warning(
                f"  Exit failed [{group.group_id}] tier {tier.tier_index}: {e}"
            )

    def _recompute_exits(self, group: TradeGroup) -> None:
        """Recompute exit plan based on current filled levels.

        Preserves triggered_exits across rebuilds since tiers are identified
        by (source_level_index, tier_index) tuples, not list positions.
        """
        filled = [
            (group.deviation_plan.levels[idx].price, shares)
            for idx, shares in sorted(group.filled_levels.items())
        ]
        if filled:
            group.exit_plan = compute_exit_plan(
                filled, group.htf_signal_price, self.num_exit_tiers,
                tier_spacing=self.tier_spacing,
            )

    # ------------------------------------------------------------------ #
    # Stop evaluation
    # ------------------------------------------------------------------ #

    def _evaluate_stops(self, htf_close: float) -> None:
        """Evaluate stop condition on all active groups."""
        for group in self.trade_groups:
            if group.status not in ("pending", "active"):
                continue
            if not group.filled_levels:
                continue

            # Check if adverse move exceeds stop
            move = (htf_close - group.htf_signal_price) / group.htf_signal_price
            stop_return = (group.stop_price - group.htf_signal_price) / group.htf_signal_price

            # Widen stop based on exit progress — groups with partial exits
            # are already profitable, so give remaining shares more room.
            total_tiers = len(group.exit_plan.tiers) if group.exit_plan else 0
            exited_tiers = len(group.triggered_exits)
            exit_progress = exited_tiers / total_tiers if total_tiers > 0 else 0.0
            widen_factor = 1.0 + exit_progress * self.stop_widen_on_exit
            effective_stop_return = stop_return * widen_factor  # more negative = wider

            if move <= effective_stop_return:
                self._stop_out_group(group, htf_close)

    def _stop_out_group(self, group: TradeGroup, current_price: float) -> None:
        """Sell all remaining shares in a group at market."""
        total_remaining = sum(group.filled_levels.values())
        # Subtract already-exited shares
        if group.exit_plan:
            for tier in group.exit_plan.tiers:
                tier_id = (tier.source_level_index, tier.tier_index)
                if tier_id in group.triggered_exits:
                    total_remaining -= tier.shares_to_sell

        server_qty = self._effective_server_position()

        if total_remaining <= 0:
            group.status = "stopped_out"
            return

        if total_remaining > server_qty:
            self.logger.warning(
                f"  Stop-out clamped [{group.group_id}]:"
                f" want to sell {total_remaining} but server_pos={server_qty},"
                f" clamping to {server_qty}"
            )
            total_remaining = server_qty
            if total_remaining <= 0:
                group.status = "stopped_out"
                return

        attributes = {
            "group_id": group.group_id,
            "action": "stop_out",
            "htf_close": f"{current_price:.2f}",
            "stop_price": f"{group.stop_price:.2f}",
        }

        try:
            self.playground.place_order(
                self.symbol,
                total_remaining,
                OrderSide.SELL,
                "equity",
                attributes=attributes,
            )
            self._sells_placed_this_tick += total_remaining
            self.logger.info(
                f"  STOP OUT [{group.group_id}]: {total_remaining} shares"
                f" @ market (HTF close=${current_price:.2f},"
                f" stop=${group.stop_price:.2f})"
            )
        except Exception as e:
            self.logger.warning(f"  Stop-out failed [{group.group_id}]: {e}")

        group.status = "stopped_out"
        self.funnel["stop_outs"] += 1

    # ------------------------------------------------------------------ #
    # End-of-simulation cleanup
    # ------------------------------------------------------------------ #

    def close_all_active_groups(self) -> None:
        """Close all remaining active groups at market (end of sim)."""
        for group in self.trade_groups:
            if group.status in ("pending", "active") and group.filled_levels:
                total_remaining = sum(group.filled_levels.values())
                if group.exit_plan:
                    for tier in group.exit_plan.tiers:
                        tier_id = (tier.source_level_index, tier.tier_index)
                        if tier_id in group.triggered_exits:
                            total_remaining -= tier.shares_to_sell
                if total_remaining > 0:
                    server_qty = self._effective_server_position()
                    if server_qty <= 0:
                        self.logger.warning(
                            f"  End-of-sim skip [{group.group_id}]:"
                            f" no server position (pos={server_qty})"
                        )
                        group.status = "closed"
                        continue
                    if total_remaining > server_qty:
                        self.logger.warning(
                            f"  End-of-sim clamped [{group.group_id}]:"
                            f" want {total_remaining} but pos={server_qty}"
                        )
                        total_remaining = server_qty
                    try:
                        self.playground.place_order(
                            self.symbol,
                            total_remaining,
                            OrderSide.SELL,
                            "equity",
                            attributes={
                                "group_id": group.group_id,
                                "action": "end_of_sim_close",
                            },
                        )
                        self._sells_placed_this_tick += total_remaining
                        self.logger.info(
                            f"  End-of-sim close [{group.group_id}]:"
                            f" {total_remaining} shares"
                        )
                    except Exception as e:
                        self.logger.warning(
                            f"  End-of-sim close failed [{group.group_id}]: {e}"
                        )
                group.status = "closed"

    # ------------------------------------------------------------------ #
    # Summary
    # ------------------------------------------------------------------ #

    def log_summary(self) -> None:
        """Log funnel summary and group statistics."""
        self.logger.info("=" * 60)
        self.logger.info("MEAN-REVERSION FUNNEL SUMMARY")
        self.logger.info("=" * 60)
        for key, val in self.funnel.items():
            self.logger.info(f"  {key:30s}: {val}")
        self.logger.info("-" * 60)

        active = sum(1 for g in self.trade_groups if g.status == "active")
        closed = sum(1 for g in self.trade_groups if g.status == "closed")
        stopped = sum(1 for g in self.trade_groups if g.status == "stopped_out")
        pending = sum(1 for g in self.trade_groups if g.status == "pending")

        self.logger.info(f"  Groups total:    {len(self.trade_groups)}")
        self.logger.info(f"  Groups active:   {active}")
        self.logger.info(f"  Groups closed:   {closed}")
        self.logger.info(f"  Groups stopped:  {stopped}")
        self.logger.info(f"  Groups pending:  {pending}")
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

def run_mean_reversion(
    playground: BacktesterPlaygroundClient,
    symbol: str,
    logger,
    pdf: PDFDocument,
    max_loss_pct: float = 0.02,
    stop_percentile: float = 0.95,
    total_shares_per_group: int = 0,
    num_exit_tiers: int = 3,
    htf_horizon: str = "1h",
    tier_spacing: str = "even",
    stop_widen_on_exit: float = 1.0,
    min_expected_profit: float = 0.0,
    ev_model: str = "distribution",
    on_tick=None,
) -> MeanReversionStrategy:
    """
    Main loop for the PDF Mean-Reversion Strategy.

    Parameters
    ----------
    on_tick : callable, optional
        Callback invoked after each tick batch with ``(strategy, tick_deltas)``.
        Can be used for periodic PDF retraining — if the callback sets
        ``strategy.pdf``, subsequent ticks use the updated PDF.

    Returns the strategy instance for report generation.
    """
    strategy = MeanReversionStrategy(
        playground, symbol, logger,
        pdf=pdf,
        max_loss_pct=max_loss_pct,
        stop_percentile=stop_percentile,
        total_shares_per_group=total_shares_per_group,
        num_exit_tiers=num_exit_tiers,
        htf_horizon=htf_horizon,
        tier_spacing=tier_spacing,
        stop_widen_on_exit=stop_widen_on_exit,
        min_expected_profit=min_expected_profit,
        ev_model=ev_model,
    )

    max_iterations = 500_000
    iteration = 0
    wall_start = time.monotonic()
    last_status_time = wall_start
    status_interval = 30  # seconds between status lines
    is_live = playground.environment == PlaygroundEnvironment.LIVE.value

    def _print_status():
        nonlocal last_status_time
        now_wall = time.monotonic()
        if now_wall - last_status_time < status_interval:
            return
        last_status_time = now_wall
        elapsed = now_wall - wall_start
        h, rem = divmod(int(elapsed), 3600)
        m, s = divmod(rem, 60)
        price = strategy._prev_ltf_bar.get("close", 0.0) if strategy._prev_ltf_bar else 0.0
        equity = playground.account.equity
        pnl = playground.get_realized_profit()
        pnl_pct = pnl / strategy.playground.account.meta.initial_balance * 100 if strategy.playground.account.meta.initial_balance else 0
        pos_qty = playground.account.get_quantity(symbol)
        active_groups = sum(1 for g in strategy.trade_groups if g.status in ("pending", "active"))
        total_groups = len(strategy.trade_groups)
        ts = playground.timestamp.strftime("%H:%M:%S") if playground.timestamp else "??:??:??"
        f = strategy.funnel
        logger.info(
            f"[STATUS] {ts} | elapsed {h:02d}h{m:02d}m | tick #{iteration}"
            f" | price ${price:.2f} | equity ${equity:,.2f}"
            f" | P&L {'+' if pnl >= 0 else ''}{pnl:,.2f} ({pnl_pct:+.1f}%)"
            f" | pos {pos_qty:.0f} shares"
            f" | groups {active_groups}/{total_groups}"
            f" | sig {f['signals_detected']} ent {f['entries_placed']}"
            f" exit {f['exits_placed']} stop {f['stop_outs']}"
        )

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

        _print_status()

        has_active = any(
            g.status in ("pending", "active") for g in strategy.trade_groups
        )

        if is_live:
            # In live mode, sleep in short increments so status updates
            # print every ~30s.  Only issue the tick RPC once the full
            # LTF period has elapsed.  playground.tick() sleeps internally
            # for the entire period, so we bypass that by sleeping here
            # and only calling tick() when it's time.
            wait_until = playground.next_tick_at
            while True:
                now = datetime.now(ZoneInfo("America/New_York"))
                if now >= wait_until:
                    break
                remaining = (wait_until - now).total_seconds()
                time.sleep(min(remaining, status_interval))
                _print_status()

            # next_tick_at has been reached — tick() will not sleep
            playground.tick(playground.ltf_seconds, fetch_account=has_active)
        else:
            # In sim mode, skip ahead by HTF when idle to reduce RPCs.
            tick_seconds = playground.ltf_seconds if has_active else playground.htf_seconds
            playground.tick(tick_seconds, fetch_account=has_active)

    # End-of-sim cleanup
    strategy.close_all_active_groups()
    strategy.log_summary()

    return strategy
