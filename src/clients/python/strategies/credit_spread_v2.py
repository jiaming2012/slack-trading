"""
Credit Spread Selling Strategy V2 -- datasource-consuming variant.

Identical to CreditSpreadStrategyV2 (V1) except _process_htf_candle() delegates
signal detection to datasources.credit_spread_signals.produce_signals() instead of
calling detect_atomic_signals_on_bar() inline. This follows the TradeSignal framework
datasource pattern established in Phase 21 (MeanReversionV2).

PDF-guided options strategy that sells credit spreads on mean-reversion signals:
- Bullish signal (stock dipped) -> sell bull put credit spread below current price
- Bearish signal (stock rallied) -> sell bear call credit spread above current price

Collects premium upfront, profits from time decay when PDF correctly predicts reversion.
Risk defined by spread width. Both directions, single-symbol.
"""

from __future__ import annotations

import uuid
from dataclasses import dataclass, field
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Set, Tuple

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
from datasources.credit_spread_signals import produce_signals
from lib.pdf_builder import _get, _get_dt
from lib.pdf_types import HorizonStats, PDFDocument, SignalPDF
from engine.types import OrderSide, SignalDecision
from rpc.playground_pb2 import GetOptionsLadderRequest
from strategies.base_strategy import BaseStrategy


# ------------------------------------------------------------------ #
# Data structures
# ------------------------------------------------------------------ #

@dataclass
class SpreadLeg:
    """One leg of a credit spread."""

    contract_symbol: str       # O:AAPL251205P00230000
    option_type: str           # "put" or "call"
    strike: float
    contracts: int
    side: str                  # "short" or "long"
    fill_price: float          # premium at fill
    expiration_date: str


@dataclass
class CreditSpreadEntry:
    """A complete credit spread at one deviation level."""

    short_leg: SpreadLeg
    long_leg: SpreadLeg
    contracts: int
    net_credit: float          # (short_premium - long_premium) * 100 * contracts
    spread_width: float        # abs(short_strike - long_strike)
    max_loss: float            # (spread_width * 100 - net_credit_per_contract * 100) * contracts
    level_index: int
    sigma_distance: float
    p_profit: float            # P(stock stays above short strike for bull put)
    expected_profit: float
    entry_stock_price: float
    entry_timestamp: datetime
    candles_held: int = 0
    dte_at_entry: Optional[int] = None


@dataclass
class CreditSpreadGroup:
    """One group of credit spreads spawned by a single HTF signal."""

    group_id: str
    htf_signal_key: str
    htf_signal_price: float
    htf_signal_timestamp: datetime
    direction: str              # "bullish" (bull put) or "bearish" (bear call)
    deviation_plan: DeviationPlan
    entries: Dict[int, CreditSpreadEntry] = field(default_factory=dict)
    exited_entries: Set[int] = field(default_factory=set)
    status: str = "pending"     # pending, active, closed
    total_credit_collected: float = 0.0
    total_collateral_used: float = 0.0
    model_name: str = "empirical"
    htf_stddev: float = 0.0
    horizons: Dict[str, HorizonStats] = field(default_factory=dict)


# ------------------------------------------------------------------ #
# Strategy class
# ------------------------------------------------------------------ #

class CreditSpreadStrategyV2(BaseStrategy):
    """
    PDF-guided credit spread selling strategy (V2 -- datasource signals).

    Sells bull put spreads on bullish dips, bear call spreads on bearish rallies.
    Risk per spread is capped at (spread_width - net_credit).
    """

    label = "credit_spread_v2"

    def __init__(
        self,
        playground: BacktesterPlaygroundClient,
        symbol: str,
        logger=None,
        pdf: PDFDocument = None,
        max_collateral_pct: float = 0.05,
        max_total_collateral_pct: float = 0.30,
        stop_percentile: float = 0.95,
        total_contracts_per_group: int = 0,
        htf_horizon: str = "1h",
        target_dte: int = 30,
        min_dte: int = 14,
        max_dte: int = 45,
        max_spread_width: float = 5.0,
        profit_target_pct: float = 1.0,
        max_loss_multiplier: float = 0.0,
        time_decay_exit_dte: int = 5,
        min_credit_per_spread: float = 0.50,
        min_hold_candles: int = 12,
        tail_threshold: float = 0.005,
        long_only: bool = False,
        min_spread_width: float = 2.50,
        max_loss_per_trade: float = 0.0,
        spread_width_sigma: float = 1.0,
        # Risk management parameters (hold-to-expiration)
        min_p_profit: float = 0.0,
        min_credit_width_ratio: float = 0.0,
        max_open_positions: int = 0,
        max_directional_imbalance: int = 0,
        max_positions_per_expiration: int = 0,
        daily_loss_limit_pct: float = 0.0,
        enable_strike_breach_exit: bool = False,
        gamma_risk_dte: int = 0,
        gamma_risk_buffer_pct: float = 0.50,
        early_profit_time_pct: float = 0.25,
        early_profit_min_pct_captured: float = 0.60,
        pre_expiration_dte: int = 1,
        enable_reversion_exit: bool = False,
        min_otm_pct: float = 0.02,
        cooldown_candles: int = 78,
        ladder_cache_minutes: int = 15,
    ):
        super().__init__(playground, symbol, logger or _default_logger)
        self.pdf = pdf
        self.max_collateral_pct = max_collateral_pct
        self.max_total_collateral_pct = max_total_collateral_pct
        self.stop_percentile = stop_percentile

        # Auto-size contracts from balance when not specified.
        # Risk per contract = spread_width * 100 shares.
        if total_contracts_per_group <= 0:
            collateral_per_contract = max_spread_width * 100
            budget = playground.account.balance * max_collateral_pct
            if collateral_per_contract > 0:
                total_contracts_per_group = max(1, int(budget / collateral_per_contract))
            else:
                total_contracts_per_group = 1
            self.logger.info(
                f"Auto-sized contracts/group: {total_contracts_per_group}"
                f" (budget ${budget:,.2f}"
                f" / collateral ${collateral_per_contract:,.2f} per contract)"
            )

        self.total_contracts_per_group = total_contracts_per_group
        self.htf_horizon = htf_horizon
        self.target_dte = target_dte
        self.min_dte = min_dte
        self.max_dte = max_dte
        self.max_spread_width = max_spread_width
        self.profit_target_pct = profit_target_pct
        self.max_loss_multiplier = max_loss_multiplier
        self.time_decay_exit_dte = time_decay_exit_dte
        self.min_credit_per_spread = min_credit_per_spread
        self.min_hold_candles = min_hold_candles
        self.tail_threshold = tail_threshold
        self.long_only = long_only
        self.max_loss_per_trade = max_loss_per_trade
        self.spread_width_sigma = spread_width_sigma
        self.min_spread_width = min_spread_width
        # Risk management
        self.min_p_profit = min_p_profit
        self.min_credit_width_ratio = min_credit_width_ratio
        self.max_open_positions = max_open_positions
        self.max_directional_imbalance = max_directional_imbalance
        self.max_positions_per_expiration = max_positions_per_expiration
        self.daily_loss_limit_pct = daily_loss_limit_pct
        self.enable_strike_breach_exit = enable_strike_breach_exit
        self.gamma_risk_dte = gamma_risk_dte
        self.gamma_risk_buffer_pct = gamma_risk_buffer_pct
        self.early_profit_time_pct = early_profit_time_pct
        self.early_profit_min_pct_captured = early_profit_min_pct_captured
        self.pre_expiration_dte = pre_expiration_dte
        self.enable_reversion_exit = enable_reversion_exit
        self.min_otm_pct = min_otm_pct
        self.cooldown_candles = cooldown_candles
        self.ladder_cache_minutes = ladder_cache_minutes

        # State
        self.trade_groups: List[CreditSpreadGroup] = []
        self._active_groups: List[CreditSpreadGroup] = []  # only pending/active
        self._prev_htf_bar: Optional[dict] = None
        self._prev_ltf_bar: Optional[dict] = None
        self._htf_signal_dedup: Set[str] = set()
        self._cached_ladder = None
        self._cached_ladder_tick_ts = None
        self._total_collateral_committed: float = 0.0
        self._open_spread_keys: Set[Tuple[str, float]] = set()  # (expiry, short_strike)
        self._positions_by_expiration: Dict[str, int] = {}
        self._daily_equity_start: Optional[float] = None
        self._current_trading_day: Optional[object] = None
        self._exit_cooldown: Dict[Tuple[str, float], int] = {}  # spread_key -> candle count at exit

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
            "entry_attempts": 0,
            "entries_placed": 0,
            "entries_skipped_no_short": 0,
            "entries_skipped_no_long": 0,
            "entries_skipped_low_credit": 0,
            "entries_skipped_debit": 0,
            "entries_skipped_collateral": 0,
            "entries_skipped_wide_spread": 0,
            "entries_skipped_neg_ev": 0,
            "entries_skipped_duplicate": 0,
            "entries_skipped_leg_failure": 0,
            "exits_pre_expiration": 0,
            "exits_strike_breach": 0,
            "exits_gamma_risk": 0,
            "exits_max_loss": 0,
            "exits_early_profit": 0,
            "exits_profit_target": 0,
            "exits_reversion": 0,
            "exits_time_decay": 0,
            "exits_end_of_sim": 0,
            "groups_closed": 0,
            "ladder_fetches": 0,
            "ladder_empty": 0,
        }

        self._log_ev_warnings()

    # ------------------------------------------------------------------ #
    # EV model diagnostics
    # ------------------------------------------------------------------ #

    def _log_ev_warnings(self) -> None:
        """Log which CLI args are modeled in EV and which are not."""
        self.logger.info("-" * 60)
        self.logger.info("EV MODEL (adaptive to CLI args)")
        self.logger.info("-" * 60)

        # Modeled parameters
        if self.profit_target_pct < 1.0:
            self.logger.info(
                f"  [EV] Profit capped at {self.profit_target_pct:.0%}"
                f" of credit (--profit-target {self.profit_target_pct})"
            )
        else:
            self.logger.info(
                "  [EV] Profit capped at 100% of credit (default)"
            )

        if self.max_loss_multiplier > 0:
            self.logger.info(
                f"  [EV] Losses capped at {self.max_loss_multiplier:.1f}x"
                f" credit (--max-loss-mult {self.max_loss_multiplier})"
            )
        else:
            self.logger.info(
                "  [EV] Losses use full expiration payoff (no stop-loss cap)"
            )

        self.logger.info("  [EV] Exit slippage estimated from bid-ask spreads")

        # Unmodeled parameters — warn
        if self.time_decay_exit_dte > 0:
            self.logger.warning(
                f"  [EV] WARNING: time_decay_exit_dte={self.time_decay_exit_dte}"
                f" — early exit not modeled, EV losses may overestimate"
            )

        if self.pre_expiration_dte > 0:
            self.logger.warning(
                f"  [EV] WARNING: pre_expiration_dte={self.pre_expiration_dte}"
                f" — positions exit before expiration"
            )

        if self.enable_strike_breach_exit:
            self.logger.warning(
                "  [EV] WARNING: strike-breach exit enabled but not modeled"
            )

        if self.gamma_risk_dte > 0:
            self.logger.warning(
                f"  [EV] WARNING: gamma_risk_dte={self.gamma_risk_dte}"
                f" — near-expiration exit not modeled"
            )

        if self.early_profit_time_pct > 0:
            self.logger.info(
                f"  [EV] Early profit exit enabled"
                f" (capture {self.early_profit_min_pct_captured:.0%}"
                f" in first {self.early_profit_time_pct:.0%} of hold)"
                f" — bounded by profit_target cap"
            )

        self.logger.info("-" * 60)

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
    # Collateral management
    # ------------------------------------------------------------------ #

    def _available_collateral(self) -> float:
        """Available collateral = max_total - committed."""
        equity = self.playground.account.equity
        return equity * self.max_total_collateral_pct - self._total_collateral_committed

    def _group_collateral_limit(self) -> float:
        """Max collateral for a single group."""
        return self.playground.account.equity * self.max_collateral_pct

    # ------------------------------------------------------------------ #
    # Risk management helpers
    # ------------------------------------------------------------------ #

    def _count_open_positions(self) -> int:
        """Count total open spread entries across all active groups."""
        count = 0
        for g in self.trade_groups:
            if g.status not in ("pending", "active"):
                continue
            count += len(g.entries) - len(g.exited_entries)
        return count

    def _directional_counts(self) -> Tuple[int, int]:
        """Return (bullish_count, bearish_count) of active groups."""
        bull = bear = 0
        for g in self.trade_groups:
            if g.status not in ("pending", "active"):
                continue
            if len(g.entries) - len(g.exited_entries) <= 0:
                continue
            if g.direction == "bullish":
                bull += 1
            else:
                bear += 1
        return bull, bear

    def _check_daily_loss_limit(self, bar_dict: dict) -> bool:
        """Return True if daily loss limit is breached (should skip entries)."""
        if self.daily_loss_limit_pct <= 0:
            return False

        current_ts = bar_dict.get("datetime")
        if current_ts is None:
            return False

        current_day = current_ts.date() if hasattr(current_ts, "date") else None
        if current_day is None:
            return False

        # Reset at start of new trading day
        if self._current_trading_day != current_day:
            self._current_trading_day = current_day
            self._daily_equity_start = self.playground.account.equity

        if self._daily_equity_start and self._daily_equity_start > 0:
            current_equity = self.playground.account.equity
            daily_loss = (self._daily_equity_start - current_equity) / self._daily_equity_start
            if daily_loss >= self.daily_loss_limit_pct:
                return True

        return False

    @staticmethod
    def _compute_dte_from_expiration(expiration_date_str: str, current_ts) -> Optional[int]:
        """Compute days to expiration from expiration date string and current timestamp."""
        try:
            exp_date = datetime.fromisoformat(
                expiration_date_str.replace("Z", "+00:00")
            ).date()
            current_date = current_ts.date() if hasattr(current_ts, "date") else current_ts
            return (exp_date - current_date).days
        except (ValueError, TypeError, AttributeError):
            return None

    def _compute_dte(self, entry: 'CreditSpreadEntry', current_ts) -> Optional[int]:
        """Compute current DTE for an entry."""
        if not entry.short_leg.expiration_date:
            return None
        return self._compute_dte_from_expiration(entry.short_leg.expiration_date, current_ts)

    def _compute_dynamic_spread_width(
        self, group: 'CreditSpreadGroup', stock_price: float, contracts: int,
    ) -> float:
        """Compute spread width from the lesser of PDF sigma and risk cap.

        1. PDF-driven: stock_price * horizon_stddev * spread_width_sigma
           (how far the model expects the stock to move)
        2. Risk-capped: max_loss_per_trade / (100 * contracts)
           (max width the per-trade loss budget allows)
        3. Result: min(pdf_width, risk_width), clamped to [min_spread_width, max_spread_width]

        Falls back to max_spread_width if no PDF data is available.
        """
        # PDF-driven width
        horizon = self._select_best_horizon(group)
        if horizon is not None and horizon.stddev > 0:
            pdf_width = stock_price * horizon.stddev * self.spread_width_sigma
        else:
            pdf_width = self.max_spread_width  # fallback

        # Risk-capped width
        if self.max_loss_per_trade > 0 and contracts > 0:
            risk_width = self.max_loss_per_trade / (100 * contracts)
        else:
            risk_width = self.max_spread_width  # fallback

        # Take the lesser, clamp to [min, max]
        target = min(pdf_width, risk_width)
        target = max(target, self.min_spread_width)
        target = min(target, self.max_spread_width)

        return target

    # ------------------------------------------------------------------ #
    # Main processing
    # ------------------------------------------------------------------ #

    def process_candles(self, new_candles) -> None:
        """Process new candles from a tick delta."""
        # Ladder cache uses time-based expiry (ladder_cache_minutes), no invalidation needed here

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
        """Handle a completed HTF candle -- V2: signals from datasource."""
        self.funnel["htf_bars"] += 1

        # V2: Use datasource instead of inline detection
        trade_signals = produce_signals(bar_dict, self._prev_htf_bar, self.pdf, self.htf_horizon)
        self._prev_htf_bar = bar_dict  # Update AFTER produce_signals (matches V1 timing)

        for sig in trade_signals:
            horizon = sig["horizon"]
            mean = horizon.mean
            stddev = horizon.stddev
            self.logger.debug(
                f"HTF signal: {sig['signal_key']} mean={mean:.5f}"
                f" stddev={stddev:.5f}"
                f" price=${bar_dict.get('close', 0):.2f}"
            )
            if sig["direction"] == "bullish":
                self._try_create_group(sig["signal_key"], sig["bar_dict"], sig["pdf_entry"], "bullish")
            elif sig["direction"] == "bearish" and not self.long_only:
                self._try_create_group(sig["signal_key"], sig["bar_dict"], sig["pdf_entry"], "bearish")

    def _process_ltf_candle(self, bar_dict: dict) -> None:
        """Handle a completed LTF candle — check entries and exits."""
        self.funnel["ltf_bars"] += 1
        self._prev_ltf_bar = bar_dict

        groups_with_new_fills: set = set()

        # Single pass over active groups only
        for group in self._active_groups:
            # Increment hold counter for open entries
            for idx, entry in group.entries.items():
                if idx not in group.exited_entries:
                    entry.candles_held += 1

            # Check entries
            fills_before = len(group.entries)
            self._check_entries(group, bar_dict)
            if len(group.entries) > fills_before:
                groups_with_new_fills.add(id(group))

            # Check exits (skip groups that just filled this candle)
            if group.status == "active" and id(group) not in groups_with_new_fills:
                self._check_exits(group, bar_dict)

        # Remove closed groups from active list
        self._active_groups = [
            g for g in self._active_groups
            if g.status in ("pending", "active")
        ]

    # ------------------------------------------------------------------ #
    # Group creation
    # ------------------------------------------------------------------ #

    def _try_create_group(
        self, signal_key: str, bar_dict: dict, pdf_entry: SignalPDF,
        direction: str,
    ) -> None:
        """Attempt to create a new credit spread trade group."""
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

        # Use large budget so deviation_levels doesn't truncate
        equity = self.playground.account.equity
        max_loss_budget = equity

        # For bearish signals, negate returns for deviation_levels
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

        group = CreditSpreadGroup(
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
        self._active_groups.append(group)
        self.funnel["groups_created"] += 1
        budget = self._group_collateral_limit()
        self.logger.info(
            f"Group {group.group_id}: {direction} signal={signal_key}"
            f" price=${signal_price:.2f} levels={len(plan.levels)}"
            f" budget=${budget:.2f} stop=${plan.stop_price:.2f}"
        )

    # ------------------------------------------------------------------ #
    # Entry checks
    # ------------------------------------------------------------------ #

    def _check_entries(self, group: CreditSpreadGroup, bar_dict: dict) -> None:
        """Check if any unfilled levels are breached."""
        if self._check_daily_loss_limit(bar_dict):
            return

        if self._available_collateral() <= 0:
            return

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
                self.logger.debug(
                    f"Level {idx} triggered: price=${level.price:.2f}"
                    f" stock_low=${low:.2f} stock_high=${high:.2f}"
                    f" sigma={level.sigma_distance:.2f}"
                )
                self._place_entry(group, idx, level, bar_dict)

    def _place_entry(
        self, group: CreditSpreadGroup, level_idx: int, level, bar_dict: dict,
    ) -> None:
        """Attempt to place a credit spread at this deviation level."""
        self.funnel["entry_attempts"] += 1
        contracts = level.shares

        # ---- CHEAP CHECKS (no ladder fetch needed) ----

        # Collateral estimate using max spread width (avoids fetching ladder)
        est_collateral = self.max_spread_width * 100 * contracts
        available_total = self._available_collateral()
        if est_collateral > available_total:
            self.funnel["entries_skipped_collateral"] += 1
            self.record_decision(SignalDecision(
                signal_type="credit_spread", direction=group.direction,
                decision="skip", reason="insufficient collateral",
                symbol="", playground_id="",
            ))
            return

        group_limit = self._group_collateral_limit()
        if group.total_collateral_used + est_collateral > group_limit:
            self.funnel["entries_skipped_collateral"] += 1
            return

        # Portfolio-level gates
        if self.max_open_positions > 0 and self._count_open_positions() >= self.max_open_positions:
            self.funnel["entries_skipped_max_positions"] = self.funnel.get("entries_skipped_max_positions", 0) + 1
            return

        if self.max_directional_imbalance > 0:
            bull, bear = self._directional_counts()
            if group.direction == "bullish" and (bull - bear) >= self.max_directional_imbalance:
                self.funnel["entries_skipped_directional"] = self.funnel.get("entries_skipped_directional", 0) + 1
                return
            if group.direction == "bearish" and (bear - bull) >= self.max_directional_imbalance:
                self.funnel["entries_skipped_directional"] = self.funnel.get("entries_skipped_directional", 0) + 1
                return

        # ---- LADDER FETCH (expensive RPC) ----

        contracts_list = self._fetch_ladder(bar_dict)
        if not contracts_list:
            self.funnel["entries_skipped_no_short"] += 1
            self.logger.debug(
                f"  SKIP [no_ladder]: No options contracts available"
                f" [{group.group_id}] level {level_idx}"
            )
            return

        stock_price = bar_dict.get("close", 0.0)

        # Compute dynamic spread width from PDF and risk cap
        dynamic_width = self._compute_dynamic_spread_width(group, stock_price, contracts)

        # Select spread strikes
        result = self._select_spread_strikes(
            level, contracts_list, group, stock_price, dynamic_width,
        )
        if result is None:
            return  # skip reason already logged and counted

        short_contract, long_contract = result
        actual_width = abs(short_contract.strike - long_contract.strike)

        # Conservative pricing: sell at bid, buy at ask
        short_bid = short_contract.bid
        long_ask = long_contract.ask

        # Check for debit spread
        if short_bid <= long_ask:
            self.funnel["entries_skipped_debit"] += 1
            self.logger.debug(
                f"  SKIP [debit]: short_bid={short_bid:.2f}"
                f" <= long_ask={long_ask:.2f},"
                f" spread is debit not credit"
                f" [{group.group_id}] level {level_idx}"
            )
            return

        net_credit_per_contract = short_bid - long_ask

        # Check bid-ask spread on both legs
        for leg_name, leg in [("short", short_contract), ("long", long_contract)]:
            mid = (leg.bid + leg.ask) / 2
            if mid > 0 and (leg.ask - leg.bid) / mid > 0.20:
                self.funnel["entries_skipped_wide_spread"] += 1
                self.logger.debug(
                    f"  SKIP [wide_spread]: {leg_name} leg bid-ask"
                    f" ratio={(leg.ask - leg.bid) / mid:.2%} > 20%"
                    f" bid={leg.bid:.2f} ask={leg.ask:.2f}"
                    f" [{group.group_id}] level {level_idx}"
                )
                return

        # Check minimum credit
        if net_credit_per_contract < self.min_credit_per_spread:
            self.funnel["entries_skipped_low_credit"] += 1
            self.logger.debug(
                f"  SKIP [low_credit]: net_credit={net_credit_per_contract:.2f}"
                f" < min={self.min_credit_per_spread:.2f}"
                f" [{group.group_id}] level {level_idx}"
            )
            return

        net_credit = net_credit_per_contract * 100 * contracts
        collateral_needed = actual_width * 100 * contracts
        max_loss = (actual_width - net_credit_per_contract) * 100 * contracts

        self.logger.debug(
            f"  Net credit: {short_bid:.2f} - {long_ask:.2f}"
            f" = {net_credit_per_contract:.2f}/spread,"
            f" total=${net_credit:.2f} for {contracts}x"
        )

        # Precise collateral check with actual width (may differ from estimate)
        if collateral_needed > available_total:
            self.funnel["entries_skipped_collateral"] += 1
            self.logger.debug(
                f"  SKIP [collateral]: need=${collateral_needed:.2f}"
                f" available=${available_total:.2f}"
                f" [{group.group_id}] level {level_idx}"
            )
            return

        if group.total_collateral_used + collateral_needed > group_limit:
            self.funnel["entries_skipped_collateral"] += 1
            self.logger.debug(
                f"  SKIP [collateral]: need=${collateral_needed:.2f}"
                f" group={group.total_collateral_used:.2f}/{group_limit:.2f}"
                f" [{group.group_id}] level {level_idx}"
            )
            return

        # Check duplicate spread
        spread_key = (short_contract.expiration_date, short_contract.strike)
        if spread_key in self._open_spread_keys:
            self.funnel["entries_skipped_duplicate"] += 1
            self.logger.debug(
                f"  SKIP [duplicate]: spread at"
                f" ({short_contract.expiration_date},"
                f" {short_contract.strike:.2f}) already open"
                f" [{group.group_id}] level {level_idx}"
            )
            return

        # Check cooldown after recent exit
        if self.cooldown_candles > 0 and spread_key in self._exit_cooldown:
            candles_since = self.funnel["ltf_bars"] - self._exit_cooldown[spread_key]
            if candles_since < self.cooldown_candles:
                self.funnel["entries_skipped_cooldown"] = self.funnel.get("entries_skipped_cooldown", 0) + 1
                self.logger.debug(
                    f"  SKIP [cooldown]: spread at"
                    f" ({short_contract.expiration_date},"
                    f" {short_contract.strike:.2f}) exited"
                    f" {candles_since} candles ago (cooldown={self.cooldown_candles})"
                    f" [{group.group_id}] level {level_idx}"
                )
                return

        if self.max_positions_per_expiration > 0:
            exp_key = short_contract.expiration_date
            current_exp_count = self._positions_by_expiration.get(exp_key, 0)
            if current_exp_count >= self.max_positions_per_expiration:
                self.funnel["entries_skipped_exp_concentration"] = self.funnel.get("entries_skipped_exp_concentration", 0) + 1
                self.logger.debug(
                    f"  SKIP [exp_concentration]: {current_exp_count}"
                    f" positions at {exp_key}"
                    f" >= {self.max_positions_per_expiration}"
                    f" [{group.group_id}] level {level_idx}"
                )
                return

        # Risk management: credit-to-width ratio
        if self.min_credit_width_ratio > 0:
            credit_width_ratio = net_credit_per_contract / actual_width
            if credit_width_ratio < self.min_credit_width_ratio:
                self.funnel["entries_skipped_thin_credit"] = self.funnel.get("entries_skipped_thin_credit", 0) + 1
                self.logger.debug(
                    f"  SKIP [thin_credit]: ratio={credit_width_ratio:.2%}"
                    f" < {self.min_credit_width_ratio:.2%}"
                    f" [{group.group_id}] level {level_idx}"
                )
                return

        # Compute probability and expected profit
        p_profit = self._compute_p_profit(
            group, short_contract.strike,
        )
        expected_profit = self._compute_expected_profit(
            group, level, short_contract, long_contract, contracts,
        )

        self.logger.debug(
            f"  EV: p_profit={p_profit:.3f}"
            f" expected_profit=${expected_profit:.2f}"
            f" (profit_cap={self.profit_target_pct:.0%}"
            f", loss_cap={'off' if self.max_loss_multiplier == 0 else f'{self.max_loss_multiplier:.1f}x'})"
        )

        # Risk management: minimum p_profit threshold
        if self.min_p_profit > 0 and p_profit < self.min_p_profit:
            self.funnel["entries_skipped_low_p_profit"] = self.funnel.get("entries_skipped_low_p_profit", 0) + 1
            self.logger.debug(
                f"  SKIP [low_p_profit]: p_profit={p_profit:.3f}"
                f" < {self.min_p_profit:.3f}"
                f" [{group.group_id}] level {level_idx}"
            )
            return

        if expected_profit < 0:
            self.funnel["entries_skipped_neg_ev"] += 1
            self.logger.debug(
                f"  SKIP [neg_ev]: expected_profit=${expected_profit:.2f} < 0"
                f" [{group.group_id}] level {level_idx}"
            )
            return

        # Place both legs
        timestamp = bar_dict.get("datetime", datetime.now())

        base_attributes = {
            "group_id": group.group_id,
            "htf_signal": group.htf_signal_key,
            "signal_price": str(group.htf_signal_price),
            "level_index": str(level_idx),
            "action": "entry",
            "direction": group.direction,
            "spread_width": f"{actual_width:.2f}",
            "net_credit": f"{net_credit:.2f}",
            "net_credit_per_contract": f"{net_credit_per_contract:.2f}",
            "max_loss": f"{max_loss:.2f}",
            "p_profit": f"{p_profit:.4f}",
            "expected_profit": f"{expected_profit:.2f}",
            "sigma_distance": f"{level.sigma_distance:.2f}",
            "model_name": group.model_name,
        }

        short_attributes = {
            **base_attributes,
            "leg": "short",
            "option_type": short_contract.type,
            "strike": f"{short_contract.strike:.2f}",
            "expiration": short_contract.expiration_date,
        }

        long_attributes = {
            **{k: v for k, v in base_attributes.items() if k != "expected_profit"},
            "leg": "long",
            "option_type": long_contract.type,
            "strike": f"{long_contract.strike:.2f}",
            "expiration": long_contract.expiration_date,
        }

        # Guard: verify neither leg has an existing position before placing
        for leg_name, leg_contract in [("short", short_contract), ("long", long_contract)]:
            if self._has_existing_position(leg_contract.symbol):
                self.funnel["entries_skipped_leg_failure"] += 1
                self.logger.warning(
                    f"  SKIP [existing_position]: {leg_name} leg"
                    f" {leg_contract.symbol} already has an open position"
                    f" [{group.group_id}] level {level_idx}"
                )
                return

        # Place short leg first (SELL_TO_OPEN)
        try:
            self.playground.place_order(
                short_contract.symbol,
                contracts,
                OrderSide.SELL_TO_OPEN,
                "option",
                short_bid,
                attributes=short_attributes,
            )
        except Exception as e:
            self.funnel["entries_skipped_leg_failure"] += 1
            self.logger.warning(
                f"  Short leg failed [{group.group_id}] level {level_idx}: {e}"
            )
            return

        # Place long leg (BUY_TO_OPEN)
        try:
            self.playground.place_order(
                long_contract.symbol,
                contracts,
                OrderSide.BUY_TO_OPEN,
                "option",
                long_ask,
                attributes=long_attributes,
            )
        except Exception as e:
            # CRITICAL: short leg is open — must close it immediately
            self.logger.warning(
                f"  Long leg failed [{group.group_id}] level {level_idx}: {e}"
                f" — closing short leg to avoid naked position"
            )
            try:
                self.playground.place_order(
                    short_contract.symbol,
                    contracts,
                    OrderSide.BUY_TO_CLOSE,
                    "option",
                    attributes={"action": "leg_failure_cleanup",
                                "group_id": group.group_id},
                )
            except Exception as e2:
                self.logger.error(
                    f"  FAILED to close orphaned short leg: {e2}"
                )
            self.funnel["entries_skipped_leg_failure"] += 1
            return

        # Both legs placed successfully
        entry = CreditSpreadEntry(
            short_leg=SpreadLeg(
                contract_symbol=short_contract.symbol,
                option_type=short_contract.type,
                strike=short_contract.strike,
                contracts=contracts,
                side="short",
                fill_price=short_bid,
                expiration_date=short_contract.expiration_date,
            ),
            long_leg=SpreadLeg(
                contract_symbol=long_contract.symbol,
                option_type=long_contract.type,
                strike=long_contract.strike,
                contracts=contracts,
                side="long",
                fill_price=long_ask,
                expiration_date=long_contract.expiration_date,
            ),
            contracts=contracts,
            net_credit=net_credit,
            spread_width=actual_width,
            max_loss=max_loss,
            level_index=level_idx,
            sigma_distance=level.sigma_distance,
            p_profit=p_profit,
            expected_profit=expected_profit,
            entry_stock_price=stock_price,
            entry_timestamp=timestamp,
            dte_at_entry=self._compute_dte_from_expiration(short_contract.expiration_date, timestamp),
        )

        group.entries[level_idx] = entry
        group.total_credit_collected += net_credit
        group.total_collateral_used += collateral_needed

        # Track positions per expiration
        exp_key = short_contract.expiration_date
        self._positions_by_expiration[exp_key] = self._positions_by_expiration.get(exp_key, 0) + 1
        self._total_collateral_committed += collateral_needed
        self._open_spread_keys.add(spread_key)
        if group.status == "pending":
            group.status = "active"
        self.funnel["entries_placed"] += 1
        self.record_decision(SignalDecision(
            signal_type="credit_spread", direction=group.direction,
            decision="place", reason=f"spread entry level {level_idx} σ={level.sigma_distance:.1f}",
            symbol="", playground_id="",
        ))

        self.logger.info(
            f"  ENTRY [{group.group_id}] level={level_idx}:"
            f" SELL {contracts}x {short_contract.symbol}"
            f" @ {short_bid:.2f},"
            f" BUY {contracts}x {long_contract.symbol}"
            f" @ {long_ask:.2f}"
            f" | credit=${net_credit:.2f} max_loss=${max_loss:.2f}"
            f" p_profit={p_profit:.3f}"
        )

        self.logger.debug(
            f"  Collateral: committed=${self._total_collateral_committed:.2f}"
            f" remaining=${self._available_collateral():.2f}"
        )

    # ------------------------------------------------------------------ #
    # Strike selection
    # ------------------------------------------------------------------ #

    def _has_existing_position(self, symbol: str) -> bool:
        """Check if there is an existing open position for the given symbol."""
        pos = self.playground.account.get_position(symbol)
        return pos is not None and pos.quantity != 0

    def _filter_contracts_without_positions(self, contracts: list) -> list:
        """Remove contracts that already have open positions."""
        return [c for c in contracts if not self._has_existing_position(c.symbol)]

    def _select_spread_strikes(
        self, level, contracts_list, group: CreditSpreadGroup,
        stock_price: float, target_width: float,
    ) -> Optional[Tuple]:
        """Select short and long strikes for the credit spread.

        Returns (short_contract, long_contract) or None if no valid pair.
        """
        if group.direction == "bullish":
            return self._select_bull_put_strikes(
                level, contracts_list, group, stock_price, target_width,
            )
        else:
            return self._select_bear_call_strikes(
                level, contracts_list, group, stock_price, target_width,
            )

    def _select_bull_put_strikes(
        self, level, contracts_list, group: CreditSpreadGroup,
        stock_price: float, target_width: float,
    ) -> Optional[Tuple]:
        """Select strikes for a bull put credit spread.

        Short put: OTM put with strike at or below level price.
        Long put: further OTM put at short_strike - target_width.
        """
        puts = [c for c in contracts_list if c.type == "put" and c.bid > 0]
        puts = self._filter_contracts_without_positions(puts)
        if not puts:
            self.funnel["entries_skipped_no_short"] += 1
            self.logger.debug(
                f"  SKIP [no_short]: No put contracts with bid > 0"
                f" (after excluding contracts with existing positions)"
                f" [{group.group_id}] level {level.sigma_distance:.2f}"
            )
            return None

        # Short put: closest to level price, at or below, and OTM by min_otm_pct
        max_short_strike = stock_price * (1 - self.min_otm_pct) if self.min_otm_pct > 0 else stock_price
        short_candidates = [c for c in puts if c.strike <= min(level.price, max_short_strike)]
        if not short_candidates:
            # Fall back: closest put below OTM threshold
            short_candidates = [c for c in puts if c.strike < max_short_strike]
        if not short_candidates:
            self.funnel["entries_skipped_no_short"] += 1
            self.logger.debug(
                f"  SKIP [no_short]: No put contracts"
                f" at strike <= {level.price:.2f}"
                f" [{group.group_id}]"
            )
            return None

        short_contract = max(short_candidates, key=lambda c: c.strike)

        # Long put: closest to short_strike - target_width
        target_long_strike = short_contract.strike - target_width
        tolerance = 2.50
        long_candidates = [
            c for c in puts
            if abs(c.strike - target_long_strike) <= tolerance
            and c.strike < short_contract.strike
        ]
        if not long_candidates:
            self.funnel["entries_skipped_no_long"] += 1
            self.logger.debug(
                f"  SKIP [no_long]: No put at"
                f" {target_long_strike:.2f} ± ${tolerance:.2f} tolerance"
                f" [{group.group_id}]"
            )
            return None

        long_contract = min(
            long_candidates, key=lambda c: abs(c.strike - target_long_strike),
        )

        # Check minimum width
        actual_width = short_contract.strike - long_contract.strike
        if actual_width < self.min_spread_width:
            self.funnel["entries_skipped_no_long"] += 1
            self.logger.debug(
                f"  SKIP [narrow]: width={actual_width:.2f}"
                f" < min={self.min_spread_width:.2f}"
                f" [{group.group_id}]"
            )
            return None

        self.logger.debug(
            f"  Selected spread: short={short_contract.strike:.2f}"
            f" bid={short_contract.bid:.2f}"
            f" long={long_contract.strike:.2f}"
            f" ask={long_contract.ask:.2f}"
            f" width={actual_width:.2f}"
        )
        return (short_contract, long_contract)

    def _select_bear_call_strikes(
        self, level, contracts_list, group: CreditSpreadGroup,
        stock_price: float, target_width: float,
    ) -> Optional[Tuple]:
        """Select strikes for a bear call credit spread.

        Short call: OTM call with strike at or above level price.
        Long call: further OTM call at short_strike + target_width.
        """
        calls = [c for c in contracts_list if c.type == "call" and c.bid > 0]
        calls = self._filter_contracts_without_positions(calls)
        if not calls:
            self.funnel["entries_skipped_no_short"] += 1
            self.logger.debug(
                f"  SKIP [no_short]: No call contracts with bid > 0"
                f" (after excluding contracts with existing positions)"
                f" [{group.group_id}] level {level.sigma_distance:.2f}"
            )
            return None

        # Short call: closest to level price, at or above, and OTM by min_otm_pct
        min_short_strike = stock_price * (1 + self.min_otm_pct) if self.min_otm_pct > 0 else stock_price
        short_candidates = [c for c in calls if c.strike >= max(level.price, min_short_strike)]
        if not short_candidates:
            short_candidates = [c for c in calls if c.strike > min_short_strike]
        if not short_candidates:
            self.funnel["entries_skipped_no_short"] += 1
            self.logger.debug(
                f"  SKIP [no_short]: No call contracts"
                f" at strike >= {level.price:.2f}"
                f" [{group.group_id}]"
            )
            return None

        short_contract = min(short_candidates, key=lambda c: c.strike)

        # Long call: closest to short_strike + target_width
        target_long_strike = short_contract.strike + target_width
        tolerance = 2.50
        long_candidates = [
            c for c in calls
            if abs(c.strike - target_long_strike) <= tolerance
            and c.strike > short_contract.strike
        ]
        if not long_candidates:
            self.funnel["entries_skipped_no_long"] += 1
            self.logger.debug(
                f"  SKIP [no_long]: No call at"
                f" {target_long_strike:.2f} ± ${tolerance:.2f} tolerance"
                f" [{group.group_id}]"
            )
            return None

        long_contract = min(
            long_candidates, key=lambda c: abs(c.strike - target_long_strike),
        )

        actual_width = long_contract.strike - short_contract.strike
        if actual_width < self.min_spread_width:
            self.funnel["entries_skipped_no_long"] += 1
            self.logger.debug(
                f"  SKIP [narrow]: width={actual_width:.2f}"
                f" < min={self.min_spread_width:.2f}"
                f" [{group.group_id}]"
            )
            return None

        self.logger.debug(
            f"  Selected spread: short={short_contract.strike:.2f}"
            f" bid={short_contract.bid:.2f}"
            f" long={long_contract.strike:.2f}"
            f" ask={long_contract.ask:.2f}"
            f" width={actual_width:.2f}"
        )
        return (short_contract, long_contract)

    # ------------------------------------------------------------------ #
    # Exit checks
    # ------------------------------------------------------------------ #

    def _check_exits(self, group: CreditSpreadGroup, bar_dict: dict) -> None:
        """Check exit conditions for an active group.

        Priority order:
        1. Pre-expiration (DTE <= pre_expiration_dte)
        2. Strike breach (underlying crossed short strike)
        3. Gamma risk (near expiration + stock near strike)
        4. Max loss (spread value stop)
        5. Early profit (fast capture)
        6. Profit target (standard theta capture)
        7. Reversion complete (if enabled)
        8. Time decay (DTE <= time_decay_exit_dte)
        """
        if group.status != "active":
            return

        close = bar_dict.get("close", 0.0)
        high = bar_dict.get("high", 0.0)
        low = bar_dict.get("low", float("inf"))

        current_ts = bar_dict.get("datetime")
        if isinstance(current_ts, str):
            from dateutil.parser import parse as parse_dt
            current_ts = parse_dt(current_ts)

        # Pre-compute reversion flag once (not per entry)
        reversion_complete = False
        if self.enable_reversion_exit:
            if group.direction == "bullish" and high >= group.htf_signal_price:
                reversion_complete = True
            elif group.direction == "bearish" and low <= group.htf_signal_price:
                reversion_complete = True

        # Single pass over all open entries
        for level_idx, entry in list(group.entries.items()):
            if level_idx in group.exited_entries:
                continue
            if entry.candles_held < self.min_hold_candles:
                continue

            dte = self._compute_dte(entry, current_ts) if current_ts else None

            # Compute spread value once per entry
            spread_value = self._compute_spread_value(entry)
            credit_per_contract = (
                entry.net_credit / (entry.contracts * 100)
                if entry.contracts > 0 else 0
            )

            # 1. PRE-EXPIRATION: close before expiration day
            if self.pre_expiration_dte > 0 and dte is not None and dte <= self.pre_expiration_dte:
                self.logger.info(
                    f"  EXIT [{group.group_id}] level={level_idx}"
                    f" reason=pre_expiration: DTE={dte}"
                )
                self._place_exit(group, entry, "pre_expiration")
                continue

            # 2. STRIKE BREACH: underlying crossed short strike (close-based)
            if self.enable_strike_breach_exit:
                if group.direction == "bullish" and close <= entry.short_leg.strike:
                    self.logger.info(
                        f"  EXIT [{group.group_id}] level={level_idx}"
                        f" reason=strike_breach:"
                        f" close=${close:.2f} <= short_strike=${entry.short_leg.strike:.2f}"
                    )
                    self._place_exit(group, entry, "strike_breach")
                    continue
                if group.direction == "bearish" and close >= entry.short_leg.strike:
                    self.logger.info(
                        f"  EXIT [{group.group_id}] level={level_idx}"
                        f" reason=strike_breach:"
                        f" close=${close:.2f} >= short_strike=${entry.short_leg.strike:.2f}"
                    )
                    self._place_exit(group, entry, "strike_breach")
                    continue

            # 3. GAMMA RISK: near expiration + stock close to short strike
            if self.gamma_risk_dte > 0 and dte is not None and dte <= self.gamma_risk_dte:
                buffer = self.gamma_risk_buffer_pct * entry.spread_width
                if group.direction == "bullish" and close <= entry.short_leg.strike + buffer:
                    self.logger.info(
                        f"  EXIT [{group.group_id}] level={level_idx}"
                        f" reason=gamma_risk: DTE={dte}"
                        f" close=${close:.2f} <= strike+buffer=${entry.short_leg.strike + buffer:.2f}"
                    )
                    self._place_exit(group, entry, "gamma_risk")
                    continue
                if group.direction == "bearish" and close >= entry.short_leg.strike - buffer:
                    self.logger.info(
                        f"  EXIT [{group.group_id}] level={level_idx}"
                        f" reason=gamma_risk: DTE={dte}"
                        f" close=${close:.2f} >= strike-buffer=${entry.short_leg.strike - buffer:.2f}"
                    )
                    self._place_exit(group, entry, "gamma_risk")
                    continue

            # 4. MAX LOSS (spread-value based) — disabled by default (0.0)
            #    Credit spreads have capped risk via spread width, so
            #    intermediate stop-outs destroy edge predicted by the model.
            if self.max_loss_multiplier > 0 and spread_value is not None:
                max_loss_threshold = self.max_loss_multiplier * credit_per_contract
                if spread_value >= max_loss_threshold:
                    self.logger.info(
                        f"  EXIT [{group.group_id}] level={level_idx}"
                        f" reason=max_loss:"
                        f" spread_value=${spread_value:.2f}"
                        f" >= threshold=${max_loss_threshold:.2f}"
                    )
                    self._place_exit(group, entry, "max_loss")
                    continue

            # 5. EARLY PROFIT: capture fast wins
            if (self.early_profit_time_pct > 0
                    and spread_value is not None
                    and entry.dte_at_entry
                    and entry.dte_at_entry > 0
                    and current_ts):
                days_held = (current_ts - entry.entry_timestamp).total_seconds() / 86400
                pct_time = days_held / entry.dte_at_entry
                if (pct_time < self.early_profit_time_pct
                        and spread_value <= (1 - self.early_profit_min_pct_captured) * credit_per_contract):
                    self.logger.info(
                        f"  EXIT [{group.group_id}] level={level_idx}"
                        f" reason=early_profit:"
                        f" {pct_time:.0%} of hold, captured"
                        f" {1 - spread_value / credit_per_contract:.0%} of credit"
                    )
                    self._place_exit(group, entry, "early_profit")
                    continue

            # 6. PROFIT TARGET
            if spread_value is not None:
                threshold = (1 - self.profit_target_pct) * credit_per_contract
                if spread_value <= threshold:
                    self.logger.info(
                        f"  EXIT [{group.group_id}] level={level_idx}"
                        f" reason=profit_target:"
                        f" spread_value=${spread_value:.2f}"
                        f" <= threshold=${threshold:.2f}"
                        f" (credit_per=${credit_per_contract:.2f})"
                    )
                    self._place_exit(group, entry, "profit_target")
                    continue

            # 7. REVERSION COMPLETE (only if enabled)
            if reversion_complete:
                self.logger.info(
                    f"  EXIT [{group.group_id}] level={level_idx}"
                    f" reason=reversion:"
                    f" spread_value=${spread_value or 0:.2f}"
                    f" vs credit=${credit_per_contract:.2f}"
                )
                self._place_exit(group, entry, "reversion")
                continue

            # 8. TIME DECAY
            if dte is not None and dte <= self.time_decay_exit_dte:
                self.logger.info(
                    f"  EXIT [{group.group_id}] level={level_idx}"
                    f" reason=time_decay: DTE={dte}"
                    f" spread_value=${spread_value or 0:.2f}"
                    f" vs credit=${credit_per_contract:.2f}"
                )
                self._place_exit(group, entry, "time_decay")
                continue

        if self._is_group_fully_exited(group):
            group.status = "closed"
            self.funnel["groups_closed"] += 1

    def _compute_spread_value(self, entry: CreditSpreadEntry) -> Optional[float]:
        """Compute current mark-to-market cost to close the spread.

        Returns the per-contract spread value (what it costs to buy back
        the short and sell the long). Lower = more profitable for seller.
        Returns None if position data unavailable.
        """
        short_pos = self.playground.account.get_position(
            entry.short_leg.contract_symbol,
        )
        long_pos = self.playground.account.get_position(
            entry.long_leg.contract_symbol,
        )

        if short_pos and long_pos:
            # Cost to close: buy back short - sell long
            return short_pos.current_price - long_pos.current_price
        elif short_pos:
            # Long leg missing — use entry price as fallback
            self.logger.debug(
                f"  Missing long position for {entry.long_leg.contract_symbol},"
                f" using entry price as fallback"
            )
            return short_pos.current_price - entry.long_leg.fill_price
        elif long_pos:
            self.logger.debug(
                f"  Missing short position for {entry.short_leg.contract_symbol},"
                f" using entry price as fallback"
            )
            return entry.short_leg.fill_price - long_pos.current_price
        else:
            self.logger.debug(
                f"  Missing both positions for spread"
                f" [{entry.short_leg.contract_symbol}/"
                f"{entry.long_leg.contract_symbol}]"
            )
            return None

    def _place_exit(
        self, group: CreditSpreadGroup, entry: CreditSpreadEntry, action: str,
    ) -> None:
        """Close both legs of a credit spread."""
        attributes = {
            "group_id": group.group_id,
            "action": f"exit_{action}",
            "level_index": str(entry.level_index),
            "direction": group.direction,
        }

        # BUY_TO_CLOSE the short leg
        try:
            short_pos = self.playground.account.get_position(
                entry.short_leg.contract_symbol,
            )
            short_qty = abs(int(short_pos.quantity)) if short_pos else entry.contracts
            if short_qty > 0:
                self.playground.place_order(
                    entry.short_leg.contract_symbol,
                    short_qty,
                    OrderSide.BUY_TO_CLOSE,
                    "option",
                    attributes={**attributes, "leg": "short"},
                )
        except Exception as e:
            self.logger.warning(
                f"  Exit short leg failed [{group.group_id}]"
                f" {entry.short_leg.contract_symbol}: {e}"
            )

        # SELL_TO_CLOSE the long leg
        try:
            long_pos = self.playground.account.get_position(
                entry.long_leg.contract_symbol,
            )
            long_qty = int(long_pos.quantity) if long_pos else entry.contracts
            if long_qty > 0:
                self.playground.place_order(
                    entry.long_leg.contract_symbol,
                    long_qty,
                    OrderSide.SELL_TO_CLOSE,
                    "option",
                    attributes={**attributes, "leg": "long"},
                )
        except Exception as e:
            self.logger.warning(
                f"  Exit long leg failed [{group.group_id}]"
                f" {entry.long_leg.contract_symbol}: {e}"
            )

        # Release collateral
        collateral = entry.spread_width * 100 * entry.contracts
        self._total_collateral_committed = max(
            0, self._total_collateral_committed - collateral,
        )
        spread_key = (entry.short_leg.expiration_date, entry.short_leg.strike)
        self._open_spread_keys.discard(spread_key)

        # Record cooldown to prevent immediate re-entry at same strikes
        if self.cooldown_candles > 0:
            self._exit_cooldown[spread_key] = self.funnel["ltf_bars"]

        # Decrement per-expiration position count
        exp_key = entry.short_leg.expiration_date
        if exp_key in self._positions_by_expiration:
            self._positions_by_expiration[exp_key] = max(0, self._positions_by_expiration[exp_key] - 1)

        group.exited_entries.add(entry.level_index)
        self.funnel[f"exits_{action}"] += 1
        self.record_decision(SignalDecision(
            signal_type="credit_spread", direction=group.direction,
            decision="place", reason=f"exit spread: {action}",
            symbol="", playground_id="",
        ))

        self.logger.debug(
            f"  Collateral: released=${collateral:.2f}"
            f" committed=${self._total_collateral_committed:.2f}"
            f" remaining=${self._available_collateral():.2f}"
        )

    def _is_group_fully_exited(self, group: CreditSpreadGroup) -> bool:
        return len(group.exited_entries) >= len(group.entries) and len(group.entries) > 0

    # ------------------------------------------------------------------ #
    # Options ladder
    # ------------------------------------------------------------------ #

    def _fetch_ladder(self, bar_dict: dict):
        """Fetch options ladder, cached for ladder_cache_minutes."""
        current_ts = bar_dict.get("datetime")
        if isinstance(current_ts, str):
            from dateutil.parser import parse as parse_dt
            current_ts = parse_dt(current_ts)

        # Check time-based cache
        if (self._cached_ladder is not None
                and self._cached_ladder_tick_ts is not None
                and current_ts is not None):
            elapsed = (current_ts - self._cached_ladder_tick_ts).total_seconds()
            if elapsed < self.ladder_cache_minutes * 60:
                return self._cached_ladder

        exp_days = self._compute_expiration_days(current_ts)
        if not exp_days:
            self.funnel["ladder_empty"] += 1
            return None

        request = GetOptionsLadderRequest(
            playground_id=self.playground.id,
            stock_symbol=self.symbol,
            max_no_of_strikes=20,
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
            self._cached_ladder_tick_ts = current_ts
            return None

        contracts = list(response.contracts)
        self._cached_ladder = contracts
        self._cached_ladder_tick_ts = current_ts
        return contracts

    def _compute_expiration_days(self, current_ts) -> list:
        """Compute candidate expiration_in_days values."""
        if current_ts is None:
            return [self.target_dte]

        candidates = []
        for dte in range(self.min_dte, self.max_dte + 1):
            future = current_ts + timedelta(days=dte)
            if future.weekday() == 4:  # Friday
                candidates.append(dte)

        if not candidates:
            return [self.target_dte]

        closest = min(candidates, key=lambda d: abs(d - self.target_dte))
        result = [closest]
        next_friday = closest + 7
        if next_friday <= self.max_dte:
            result.append(next_friday)
        return result

    # ------------------------------------------------------------------ #
    # Probability and expected profit
    # ------------------------------------------------------------------ #

    # Approximate calendar-day equivalents for each horizon name.
    _HORIZON_DAYS: Dict[str, float] = {
        "1h": 1 / 6.5,
        "4h": 4 / 6.5,
        "1d": 1.0,
        "2d": 2.0,
        "1w": 5.0,
        "2w": 10.0,
        "1m": 21.0,
    }

    @staticmethod
    def _horizon_sort_key(name: str) -> int:
        _KNOWN = {
            "1h": 4, "4h": 16, "1d": 26, "2d": 52,
            "1w": 130, "2w": 260, "1m": 520,
        }
        return _KNOWN.get(name, 0)

    def _select_best_horizon(
        self, group: CreditSpreadGroup,
        target_days: Optional[float] = None,
    ) -> Optional[HorizonStats]:
        """Select the horizon closest to target_days.

        If target_days is None, defaults to the strategy's target_dte
        (matching the option expiration horizon). Falls back to the
        longest available horizon if no close match exists.
        """
        if target_days is None:
            target_days = float(self.target_dte)

        best = None
        best_distance = float("inf")
        for name, stats in group.horizons.items():
            if not stats.forward_returns:
                continue
            horizon_days = self._HORIZON_DAYS.get(name)
            if horizon_days is None:
                continue
            distance = abs(horizon_days - target_days)
            if distance < best_distance:
                best_distance = distance
                best = stats
        return best

    def _compute_p_profit(
        self, group: CreditSpreadGroup, short_strike: float,
    ) -> float:
        """Probability the spread expires profitable.

        Bull put: P(exit_price >= short_strike) — stock stays above.
        Bear call: P(exit_price <= short_strike) — stock stays below.
        """
        horizon = self._select_best_horizon(group)
        if horizon is None or not horizon.forward_returns:
            return 0.5  # neutral fallback

        returns = np.array(horizon.forward_returns)
        exit_prices = group.htf_signal_price * (1.0 + returns)

        if group.direction == "bullish":
            return float(np.mean(exit_prices >= short_strike))
        else:
            return float(np.mean(exit_prices <= short_strike))

    def _estimate_exit_slippage(self, short_contract, long_contract) -> float:
        """Estimate per-contract cost of closing the spread due to bid-ask.

        Uses entry-time bid-ask spreads as a proxy for exit-time spreads.
        Returns a positive number (cost to subtract from P&L).
        """
        short_spread = max(0, short_contract.ask - short_contract.bid)
        long_spread = max(0, long_contract.ask - long_contract.bid)
        return (short_spread + long_spread) / 2.0

    def _compute_expected_profit(
        self, group: CreditSpreadGroup, level, short_contract, long_contract,
        contracts: int,
    ) -> float:
        """Compute exit-aware expected profit by integrating over forward returns.

        Adapts to CLI args to act as a conservative lower bound:
        - Caps wins at ``profit_target_pct * net_credit`` (can't capture more)
        - Caps losses at ``max_loss_multiplier * net_credit`` (if enabled)
        - Subtracts estimated exit slippage from all scenarios

        For each return scenario:
        - Stock above short strike → full profit (keep net credit)
        - Stock below long strike → max loss
        - Stock between strikes → partial loss
        """
        horizon = self._select_best_horizon(group)
        if horizon is None or not horizon.forward_returns:
            return 0.0

        signal_price = group.htf_signal_price
        short_strike = short_contract.strike
        long_strike = long_contract.strike
        net_credit_per = short_contract.bid - long_contract.ask
        actual_width = abs(short_strike - long_strike)

        returns = np.array(horizon.forward_returns)
        exit_prices = signal_price * (1.0 + returns)

        if group.direction == "bullish":
            # Bull put: profit when stock above short strike
            pl_per_contract = np.where(
                exit_prices >= short_strike,
                net_credit_per,  # full profit
                np.where(
                    exit_prices <= long_strike,
                    net_credit_per - actual_width,  # max loss
                    net_credit_per - (short_strike - exit_prices),  # partial
                ),
            )
        else:
            # Bear call: profit when stock below short strike
            pl_per_contract = np.where(
                exit_prices <= short_strike,
                net_credit_per,  # full profit
                np.where(
                    exit_prices >= long_strike,
                    net_credit_per - actual_width,  # max loss
                    net_credit_per - (exit_prices - short_strike),  # partial
                ),
            )

        # --- Exit-aware adjustments (adapt to CLI args) ---

        # 1. Cap wins at profit_target_pct of credit
        profit_cap = self.profit_target_pct * net_credit_per
        pl_per_contract = np.where(
            pl_per_contract > 0,
            np.minimum(pl_per_contract, profit_cap),
            pl_per_contract,
        )

        # 2. Cap losses at max_loss_multiplier * credit (if enabled)
        if self.max_loss_multiplier > 0:
            loss_floor = -self.max_loss_multiplier * net_credit_per
            pl_per_contract = np.where(
                pl_per_contract < 0,
                np.maximum(pl_per_contract, loss_floor),
                pl_per_contract,
            )

        # 3. Subtract exit slippage (always)
        exit_slippage = self._estimate_exit_slippage(short_contract, long_contract)
        pl_per_contract = pl_per_contract - exit_slippage

        return float(np.mean(pl_per_contract)) * 100 * contracts

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
                self._place_exit(group, entry, "end_of_sim")
            group.status = "closed"

    # ------------------------------------------------------------------ #
    # Summary
    # ------------------------------------------------------------------ #

    def log_summary(self) -> None:
        """Log funnel summary and group statistics."""
        self.logger.info("=" * 60)
        self.logger.info("CREDIT SPREAD STRATEGY FUNNEL SUMMARY")
        self.logger.info("=" * 60)
        for key, val in self.funnel.items():
            self.logger.info(f"  {key:35s}: {val}")
        self.logger.info("-" * 60)

        active = sum(1 for g in self.trade_groups if g.status == "active")
        closed = sum(1 for g in self.trade_groups if g.status == "closed")
        pending = sum(1 for g in self.trade_groups if g.status == "pending")
        bullish = sum(1 for g in self.trade_groups if g.direction == "bullish")
        bearish = sum(1 for g in self.trade_groups if g.direction == "bearish")

        total_credit = sum(g.total_credit_collected for g in self.trade_groups)
        total_collateral = sum(g.total_collateral_used for g in self.trade_groups)

        self.logger.info(f"  Groups total:       {len(self.trade_groups)}")
        self.logger.info(f"  Groups bullish:     {bullish}")
        self.logger.info(f"  Groups bearish:     {bearish}")
        self.logger.info(f"  Groups active:      {active}")
        self.logger.info(f"  Groups closed:      {closed}")
        self.logger.info(f"  Groups pending:     {pending}")
        self.logger.info(f"  Total credit:       ${total_credit:,.2f}")
        self.logger.info(f"  Total collateral:   ${total_collateral:,.2f}")
        self.logger.info(f"  Collateral in use:  ${self._total_collateral_committed:,.2f}")

        # Per-group P&L summary
        self.logger.info("-" * 60)
        self.logger.info("  GROUP P&L SUMMARY")
        self.logger.info(f"  {'GID':>8s} {'DIR':>7s} {'STATUS':>8s}"
                         f" {'ENTRIES':>7s} {'CREDIT':>10s} {'COLLAT':>10s}")
        for g in self.trade_groups:
            self.logger.info(
                f"  {g.group_id:>8s} {g.direction:>7s} {g.status:>8s}"
                f" {len(g.entries):>7d}"
                f" ${g.total_credit_collected:>9,.2f}"
                f" ${g.total_collateral_used:>9,.2f}"
            )
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

def run_credit_spread_v2(
    playground: BacktesterPlaygroundClient,
    symbol: str,
    logger,
    pdf: PDFDocument,
    max_collateral_pct: float = 0.05,
    max_total_collateral_pct: float = 0.30,
    stop_percentile: float = 0.95,
    total_contracts_per_group: int = 0,
    htf_horizon: str = "1h",
    target_dte: int = 30,
    min_dte: int = 14,
    max_dte: int = 45,
    max_spread_width: float = 5.0,
    profit_target_pct: float = 1.0,
    max_loss_multiplier: float = 2.0,
    time_decay_exit_dte: int = 5,
    min_credit_per_spread: float = 0.50,
    min_hold_candles: int = 12,
    tail_threshold: float = 0.005,
    long_only: bool = False,
    max_loss_per_trade: float = 0.0,
    spread_width_sigma: float = 1.0,
    on_tick=None,
    # Risk management parameters
    min_p_profit: float = 0.0,
    min_credit_width_ratio: float = 0.0,
    max_open_positions: int = 0,
    max_directional_imbalance: int = 0,
    max_positions_per_expiration: int = 0,
    daily_loss_limit_pct: float = 0.0,
    enable_strike_breach_exit: bool = False,
    gamma_risk_dte: int = 0,
    gamma_risk_buffer_pct: float = 0.50,
    early_profit_time_pct: float = 0.25,
    early_profit_min_pct_captured: float = 0.60,
    pre_expiration_dte: int = 1,
    enable_reversion_exit: bool = False,
    min_otm_pct: float = 0.02,
    cooldown_candles: int = 78,
    ladder_cache_minutes: int = 15,
) -> CreditSpreadStrategyV2:
    """
    Main loop for the Credit Spread Strategy.

    Returns the strategy instance.
    """
    strategy = CreditSpreadStrategyV2(
        playground, symbol, logger,
        pdf=pdf,
        max_collateral_pct=max_collateral_pct,
        max_total_collateral_pct=max_total_collateral_pct,
        stop_percentile=stop_percentile,
        total_contracts_per_group=total_contracts_per_group,
        htf_horizon=htf_horizon,
        target_dte=target_dte,
        min_dte=min_dte,
        max_dte=max_dte,
        max_spread_width=max_spread_width,
        profit_target_pct=profit_target_pct,
        max_loss_multiplier=max_loss_multiplier,
        time_decay_exit_dte=time_decay_exit_dte,
        min_credit_per_spread=min_credit_per_spread,
        min_hold_candles=min_hold_candles,
        tail_threshold=tail_threshold,
        long_only=long_only,
        max_loss_per_trade=max_loss_per_trade,
        spread_width_sigma=spread_width_sigma,
        min_p_profit=min_p_profit,
        min_credit_width_ratio=min_credit_width_ratio,
        max_open_positions=max_open_positions,
        max_directional_imbalance=max_directional_imbalance,
        max_positions_per_expiration=max_positions_per_expiration,
        daily_loss_limit_pct=daily_loss_limit_pct,
        enable_strike_breach_exit=enable_strike_breach_exit,
        gamma_risk_dte=gamma_risk_dte,
        gamma_risk_buffer_pct=gamma_risk_buffer_pct,
        early_profit_time_pct=early_profit_time_pct,
        early_profit_min_pct_captured=early_profit_min_pct_captured,
        pre_expiration_dte=pre_expiration_dte,
        enable_reversion_exit=enable_reversion_exit,
        min_otm_pct=min_otm_pct,
        cooldown_candles=cooldown_candles,
        ladder_cache_minutes=ladder_cache_minutes,
    )

    max_iterations = 500_000
    iteration = 0
    last_progress_date = None

    while not playground.is_backtest_complete():
        iteration += 1
        if iteration > max_iterations:
            logger.warning(f"Max iterations ({max_iterations}) reached, stopping")
            break

        tick_deltas = playground.flush_new_state_buffer()
        _profiler = playground.profiler
        for tick_delta in tick_deltas:
            new_candles = (
                tick_delta.new_candles
                if hasattr(tick_delta, "new_candles")
                else []
            )
            if _profiler:
                with _profiler.track("process_candles"):
                    strategy.process_candles(new_candles)
            else:
                strategy.process_candles(new_candles)

        if on_tick is not None:
            if _profiler:
                with _profiler.track("on_tick_callback"):
                    on_tick(strategy, tick_deltas)
            else:
                on_tick(strategy, tick_deltas)

        # Progress indicator: log once per simulated day
        if playground.timestamp:
            current_date = playground.timestamp.date()
            if current_date != last_progress_date:
                active_groups = sum(
                    1 for g in strategy.trade_groups if g.status == "active"
                )
                logger.info(
                    f"[{current_date}] tick={iteration}"
                    f" groups={len(strategy.trade_groups)}"
                    f" active={active_groups}"
                    f" entries={strategy.funnel.get('entries_placed', 0)}"
                    f" equity=${playground.account.equity:,.0f}"
                )
                last_progress_date = current_date

        # When idle (no open/pending groups), tick by HTF period instead of LTF
        # to skip ~12x fewer RPCs. The server returns all completed candles in
        # the interval, so HTF signals and LTF entries are still processed.
        has_active = len(strategy._active_groups) > 0
        tick_seconds = playground.ltf_seconds if has_active else playground.htf_seconds
        playground.tick(tick_seconds, fetch_account=has_active)

    # End-of-sim cleanup
    strategy.close_all_active_groups()
    strategy.log_summary()

    return strategy
