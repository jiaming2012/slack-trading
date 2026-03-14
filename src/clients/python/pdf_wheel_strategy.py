"""
PDF-Guided Wheel Strategy

Extends the base WheelStrategy with empirical probability distributions
(PDFs) to guide put strike selection, position sizing, and signal detection.

Key differences from vanilla WheelStrategy:
  - Uses 15-min LTF candles (not 1-hour)
  - Detects compound signals (candlestick + indicator combos) via pdf_builder
  - Sells puts at multiple strike levels (20%–50% probability) simultaneously
  - Sizes positions via Kelly criterion with drawdown scaling
  - PDF lookup replaces supertrend-only signal generation

Phase 1 (SELL_PUTS):  No stock held.  Detect compound signals, look up PDF,
                      sell puts at multiple OTM strikes based on PDF percentiles.
Phase 2 (SELL_CALLS): Stock held.  Sell covered calls (delegates to parent).
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Dict, List, Optional, Tuple

from dateutil.parser import isoparse
from rpc.playground_pb2 import (
    GetOptionsLadderRequest,
    OptionLadderContract,
    Candle,
)

from backtester_playground_client_grpc import (
    BacktesterPlaygroundClient,
    Repository,
    OrderSide,
)
from pdf_builder import detect_atomic_signals_on_bar, _get, _get_dt
from pdf_types import PDFDocument, SignalPDF, CompoundSignal
from risk_management import (
    kelly_fraction,
    adjusted_kelly,
    max_contracts,
    allocate_contracts,
)
from wheel_strategy import WheelStrategy, WheelPhase
from options_strategy_basic_v7 import (
    CloseSignalV2,
    OpenSignalV4,
    RollSignalV1,
    calculate_expected_profit_binomial_american,
    calculate_stock_quantity,
    generate_signal_stats,
)


# ------------------------------------------------------------------ #
# PDF-specific signal type
# ------------------------------------------------------------------ #

@dataclass
class PDFPutSignal:
    """Signal to sell puts at multiple strikes guided by PDF distributions."""

    symbol: str
    name: str
    timestamp: datetime
    price: float
    compound_key: str
    pdf_entry: SignalPDF
    strikes: List[StrikeAllocation] = field(default_factory=list)


@dataclass
class StrikeAllocation:
    """A single strike level with its allocated contract count."""

    strike: float
    contracts: int
    probability: float          # probability of expiring OTM
    contract_symbol: str = ""   # filled after matching to options ladder


# ------------------------------------------------------------------ #
# PDF-Guided Wheel Strategy
# ------------------------------------------------------------------ #

class PDFWheelStrategy(WheelStrategy):
    """
    Wheel strategy that uses empirical PDF distributions to select
    put strikes and size positions.

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
    peak_equity : float
        High-water-mark equity for drawdown scaling.
    kelly_fraction_mult : float
        Kelly multiplier (default 0.5 = half-Kelly).
    probability_levels : list[float]
        Target OTM probabilities for strike selection (default [0.2, 0.3, 0.4, 0.5]).
    max_open_count : int
        Max concurrent short option positions (default 5).
    signal_ci_threshold : float
        Maximum 95% CI width for a signal to be considered usable.
        Default 0.01 (relaxed from the PDF builder's 0.005 to allow
        more signals through — Kelly sizing limits exposure on
        lower-confidence entries).
    """

    def __init__(
        self,
        playground: BacktesterPlaygroundClient,
        symbol: str,
        logger,
        pdf: PDFDocument,
        peak_equity: float = 0.0,
        kelly_fraction_mult: float = 0.5,
        probability_levels: Optional[List[float]] = None,
        max_open_count: int = 5,
        signal_ci_threshold: float = 0.01,
    ):
        super().__init__(playground, symbol, logger, max_open_count=max_open_count)
        self.pdf = pdf
        self.peak_equity = peak_equity or playground.account.equity
        self.kelly_fraction_mult = kelly_fraction_mult
        self.probability_levels = probability_levels or [0.20, 0.30, 0.40, 0.50]
        self.signal_ci_threshold = signal_ci_threshold

        # Track daily context signals (date_str → [signal_names])
        self._daily_signals: Dict[str, List[str]] = {}
        self._prev_ltf_bar: Optional[dict] = None
        self._prev_daily_bar: Optional[dict] = None

    # ------------------------------------------------------------------ #
    # Repository configuration (15-min + daily)
    # ------------------------------------------------------------------ #

    @classmethod
    def get_repositories(
        cls,
        symbol: str,
    ) -> List[Repository]:
        """
        15-min LTF + 1-day HTF repositories with indicators for
        compound signal detection.
        """
        indicators = [
            "supertrend", "stochrsi", "atr", "doji", "hammer",
            "50_sma", "100_sma", "200_sma",
            "stochrsi_cross_above_20", "stochrsi_cross_below_80",
        ]
        return [
            Repository(
                symbol=symbol,
                timespan_multiplier=15,
                timespan_unit="minute",
                indicators=indicators,
                history_in_days=365,
            ),
            Repository(
                symbol=symbol,
                timespan_multiplier=1,
                timespan_unit="day",
                indicators=indicators,
                history_in_days=365,
            ),
        ]

    # ------------------------------------------------------------------ #
    # Signal detection on live bars
    # ------------------------------------------------------------------ #

    def _bar_to_dict(self, bar) -> dict:
        """Convert a protobuf Bar (or dict-like) to a plain dict for pdf_builder functions."""
        if isinstance(bar, dict):
            return bar
        # Protobuf Candle bar object
        d = {}
        for f in ("open", "high", "low", "close", "datetime",
                   "superD_50_3", "stochrsi_k_14_14_3_3",
                   "stochrsi_cross_above_20", "stochrsi_cross_below_80",
                   "cdl_hammer", "cdl_doji_10_0_1",
                   "sma_50", "sma_100", "sma_200"):
            val = getattr(bar, f, None)
            if val is not None:
                d[f] = val
        return d

    def detect_signals_on_bar(
        self,
        bar_dict: dict,
        prev_bar_dict: Optional[dict],
        timeframe: str = "ltf",
    ) -> List[str]:
        """Detect atomic signals on a single bar using pdf_builder logic."""
        return detect_atomic_signals_on_bar(
            bar_dict, prev_bar=prev_bar_dict, timeframe=timeframe,
        )

    # ------------------------------------------------------------------ #
    # Override: skip puts in roll signal check (parent only supports calls)
    # ------------------------------------------------------------------ #

    def check_for_roll_signal(self, option_prices: dict) -> list:
        """
        Override to skip put positions before checking for roll signals.

        The parent OptionsStrategyBasic.check_for_roll_signal raises on puts.
        We replicate the parent logic but skip puts instead of raising.
        """
        previous_candle = self.candles_ltf.iloc[self.candles_ltf_idx - 1]
        if previous_candle is None:
            return []

        signals = []
        positions = self.playground.get_option_positions()
        last_price = previous_candle.close

        for key in positions:
            position = positions[key]
            if position.quantity >= 0:
                continue

            contract = self.option_contract_repo.get_contract_details(position.symbol)
            if contract.option_type != "call":
                continue  # skip puts instead of raising

            if last_price < contract.strike_price:
                continue

            option_price = option_prices.get(position.symbol, None)
            if option_price is None:
                continue

            hours_to_expiration = (
                contract.expiration_date - self.playground.timestamp
            ).total_seconds() / 3600.0
            intrinsic_value = self._get_intrinsic_value(
                contract.option_type, contract.strike_price, last_price,
            )
            intrinsic_potential = intrinsic_value / option_price if option_price > 0 else 0.0
            extrinsic_value_ratio = hours_to_expiration / intrinsic_potential if intrinsic_potential > 0 else float("inf")

            signal_name = None
            if extrinsic_value_ratio < 1.2:
                signal_name = f"ROLL_CALL_EVR_{extrinsic_value_ratio:.2f}"

            loss_ratio = (position.cost_basis - position.current_price) / position.cost_basis if position.cost_basis > 0 else 0.0
            if loss_ratio <= -0.25:
                signal_name = f"ROLL_CALL_LOSS_RATIO_{loss_ratio:.2f}"

            if signal_name is None:
                continue

            result = self._get_feature_vector(previous_candle)
            if result is None:
                continue

            ltf_supertrend_count, ltf_supertrend_value, st_direction = result

            signals.append(
                RollSignalV1(
                    name=signal_name,
                    symbol=contract.underlying_symbol,
                    timestamp=self.playground.timestamp,
                    price=last_price,
                    intrinsic_value=intrinsic_value,
                    option_price=option_price,
                    days_to_expiration=hours_to_expiration / 24.0,
                    option_contract=contract,
                    quantity_to_roll=abs(position.quantity),
                    ltf_supertrend_count=ltf_supertrend_count,
                    ltf_supertrend_value=ltf_supertrend_value,
                    ltf_supertrend_direction=st_direction,
                )
            )

        return signals

    def _update_daily_context(self, candle) -> None:
        """
        If the candle is a daily bar, detect signals and store them
        keyed by date for compound signal construction.
        """
        bar_dict = self._bar_to_dict(candle.bar)
        sigs = self.detect_signals_on_bar(
            bar_dict, self._prev_daily_bar, timeframe="daily",
        )
        dt = _get_dt(bar_dict)
        if dt:
            self._daily_signals[dt.strftime("%Y-%m-%d")] = sigs
        self._prev_daily_bar = bar_dict

    def _build_compound_key(self, ltf_sigs: List[str], date_key: str) -> str:
        """Merge LTF signals with daily context and return a sorted pipe-delimited key."""
        daily_sigs = self._daily_signals.get(date_key, [])
        all_components = sorted(set(ltf_sigs + daily_sigs))
        return "|".join(all_components)

    # ------------------------------------------------------------------ #
    # Strike selection from PDF
    # ------------------------------------------------------------------ #

    def select_put_strikes(
        self,
        pdf_entry: SignalPDF,
        current_price: float,
        contracts: list,
        total_contracts: int,
    ) -> List[StrikeAllocation]:
        """
        Pick put strikes at each probability level from the PDF percentiles,
        then allocate contracts proportionally.

        For each target probability P (e.g. 0.20, 0.30, 0.40, 0.50):
          - percentile key = str(int((1 - P) * 100))  → "80", "70", "60", "50"
          - price_drop = pdf_entry.horizons[horizon].percentiles[pct_key]
          - target_strike = current_price * (1 + price_drop)
          - Find closest available OTM put from options ladder

        Parameters
        ----------
        pdf_entry : SignalPDF
            Distribution data for the detected compound signal.
        current_price : float
            Current stock price.
        contracts : list
            Available option contracts from the ladder.
        total_contracts : int
            Total contracts to distribute across strikes.

        Returns
        -------
        list[StrikeAllocation]
            Allocated strikes with contract counts.
        """
        if total_contracts <= 0:
            return []

        # Use the longest available horizon for strike selection
        horizon_preference = ["2d", "1d", "4h", "1h"]
        horizon = None
        for h in horizon_preference:
            if h in pdf_entry.horizons:
                horizon = pdf_entry.horizons[h]
                break
        if horizon is None:
            return []

        # Collect available OTM put strikes
        put_strikes: List[OptionLadderContract] = []
        for c in contracts:
            if hasattr(c, "type") and c.type != "put":
                continue
            if c.strike >= current_price:
                continue  # skip ITM/ATM
            put_strikes.append(c)

        if not put_strikes:
            return []

        put_strikes.sort(key=lambda c: c.strike, reverse=True)  # highest first

        # Map each probability level to a target strike
        targets: List[Tuple[float, float, Optional[OptionLadderContract]]] = []
        for prob in self.probability_levels:
            pct_key = str(int((1 - prob) * 100))
            price_drop = horizon.percentiles.get(pct_key, None)
            if price_drop is None:
                continue
            target_strike = current_price * (1 + price_drop)

            # Find closest available put strike
            best = None
            best_diff = float("inf")
            for ps in put_strikes:
                diff = abs(ps.strike - target_strike)
                if diff < best_diff:
                    best_diff = diff
                    best = ps
            if best is not None:
                targets.append((prob, best.strike, best))

        if not targets:
            return []

        # Deduplicate: if multiple probs map to same strike, keep highest prob
        seen_strikes: Dict[float, Tuple[float, OptionLadderContract]] = {}
        for prob, strike, contract in targets:
            if strike not in seen_strikes or prob > seen_strikes[strike][0]:
                seen_strikes[strike] = (prob, contract)

        unique_targets = sorted(seen_strikes.items(), key=lambda x: x[0], reverse=True)
        probs = [v[0] for _, v in unique_targets]
        ladder_contracts = [v[1] for _, v in unique_targets]

        # Allocate contracts
        alloc = allocate_contracts(total_contracts, probs)

        allocations = []
        for i, (strike, (prob, contract)) in enumerate(unique_targets):
            if alloc[i] > 0:
                allocations.append(StrikeAllocation(
                    strike=strike,
                    contracts=alloc[i],
                    probability=prob,
                    contract_symbol=contract.symbol if hasattr(contract, "symbol") else "",
                ))

        return allocations

    # ------------------------------------------------------------------ #
    # Position sizing via Kelly criterion
    # ------------------------------------------------------------------ #

    def compute_total_contracts(
        self,
        pdf_entry: SignalPDF,
        current_price: float,
    ) -> int:
        """
        Use Kelly criterion + drawdown scaling to determine total contracts.

        The win probability comes from the PDF's forward return distribution:
        fraction of returns > 0.
        Win/loss ratio = mean_win / |mean_loss|.
        """
        # Use longest horizon
        horizon_preference = ["2d", "1d", "4h", "1h"]
        returns = None
        for h in horizon_preference:
            if h in pdf_entry.horizons:
                returns = pdf_entry.horizons[h].forward_returns
                break
        if not returns or len(returns) < 5:
            return 0

        wins = [r for r in returns if r > 0]
        losses = [r for r in returns if r <= 0]

        if not wins or not losses:
            return 0

        win_prob = len(wins) / len(returns)
        mean_win = sum(wins) / len(wins)
        mean_loss = abs(sum(losses) / len(losses))

        if mean_loss == 0:
            return 0

        win_loss_ratio = mean_win / mean_loss
        f_star = kelly_fraction(win_prob, win_loss_ratio)

        current_equity = self.playground.account.equity
        adj_f = adjusted_kelly(
            f_star, current_equity, self.peak_equity,
            fraction=self.kelly_fraction_mult,
        )

        # Margin per contract: strike × 100
        margin = current_price * 100
        total = max_contracts(adj_f, current_equity, margin)

        self.logger.debug(
            f"[kelly] p={win_prob:.2f} b={win_loss_ratio:.2f} f*={f_star:.4f}"
            f" adj_f={adj_f:.4f} margin=${margin:.0f} → {total} contracts"
        )

        # If Kelly is positive (edge exists) but margin floors to 0,
        # allow at least 1 contract so we don't miss every signal
        if total == 0 and f_star > 0:
            total = 1
            self.logger.debug(
                f"[kelly] Edge exists (f*={f_star:.4f}) — minimum 1 contract"
            )

        # Update peak equity
        if current_equity > self.peak_equity:
            self.peak_equity = current_equity

        return total

    # ------------------------------------------------------------------ #
    # Override: Phase 1 tick with PDF-guided put signals
    # ------------------------------------------------------------------ #

    def check_for_pdf_put_signals(self, bar_dict: dict) -> Optional[PDFPutSignal]:
        """
        Detect compound signals on the current bar, look up the PDF,
        and return a PDFPutSignal if the distribution has sufficient data.

        Uses subset matching: if the full compound key isn't in the PDF,
        tries progressively smaller subsets (down to individual signals)
        and picks the most specific match with sufficient data.
        """
        ltf_sigs = self.detect_signals_on_bar(
            bar_dict, self._prev_ltf_bar, timeframe="ltf",
        )
        if not ltf_sigs:
            return None

        dt = _get_dt(bar_dict)
        if dt is None:
            return None

        date_key = dt.strftime("%Y-%m-%d")
        daily_sigs = self._daily_signals.get(date_key, [])
        all_components = sorted(set(ltf_sigs + daily_sigs))

        # Try exact match first, then subsets down to single signals
        result = self.pdf.find_best_signal(
            all_components,
            min_ci_width=self.signal_ci_threshold,
            min_components=1,
        )
        if result is None:
            self.logger.debug(
                f"[funnel] No PDF match for signals: {all_components}"
            )
            return None

        matched_key, pdf_entry = result
        price = float(_get(bar_dict, "close"))

        return PDFPutSignal(
            symbol=self.symbol,
            name="PDF_SHORT_PUT_SIGNAL",
            timestamp=dt,
            price=price,
            compound_key=matched_key,
            pdf_entry=pdf_entry,
        )

    def tick(self, tick_delta) -> Tuple[List, List[CloseSignalV2]]:
        """
        Override tick to use PDF-guided compound signal detection
        for Phase 1 (put selling).  Phase 2 delegates to parent.
        """
        phase = self.get_current_phase()

        if phase == WheelPhase.SELL_CALLS:
            self.logger.debug("PDF Wheel Phase 2: Selling Covered Calls")
            return super().tick(tick_delta)  # WheelStrategy.tick handles Phase 2

        # ---- Phase 1: PDF-guided put selling ----
        self.logger.debug("PDF Wheel Phase 1: Selling PDF-Guided Puts")

        open_signals: List[PDFPutSignal] = []
        new_candles = (
            tick_delta.new_candles if hasattr(tick_delta, "new_candles") else []
        )

        for c in new_candles:
            # Process daily candles for context
            if hasattr(c, "period") and c.period == 86400:
                self._update_daily_context(c)
                continue

            # Only process LTF candles for the target symbol
            if hasattr(c, "period") and c.period != self.playground.ltf_seconds:
                continue
            if hasattr(c, "symbol") and c.symbol != self.symbol:
                continue

            bar_dict = self._bar_to_dict(c.bar)

            open_qty = self.playground.get_options_quantity(self.symbol)
            if abs(open_qty) < self.max_open_count:
                signal = self.check_for_pdf_put_signals(bar_dict)
                if signal:
                    open_signals.append(signal)
            else:
                self.logger.debug(
                    f"[funnel] At max positions ({self.max_open_count}), skipping signal check"
                )

            self._prev_ltf_bar = bar_dict
            self.append_candle(c.bar)

        close_signals = self.check_for_early_close_puts()
        return open_signals, close_signals


# ------------------------------------------------------------------ #
# Strategy runner
# ------------------------------------------------------------------ #

def run_pdf_wheel_strategy(
    playground: BacktesterPlaygroundClient,
    symbol: str,
    logger,
    twirp_host: str,
    pdf: PDFDocument,
    peak_equity: float = 0.0,
    kelly_fraction_mult: float = 0.5,
    probability_levels: Optional[List[float]] = None,
    signal_ci_threshold: float = 0.01,
) -> None:
    """
    Main loop for the PDF-Guided Wheel Strategy.

    Parameters
    ----------
    playground : BacktesterPlaygroundClient
        The backtester playground client.
    symbol : str
        Underlying stock symbol.
    logger : loguru.Logger
        Logger instance.
    twirp_host : str
        Twirp server URL.
    pdf : PDFDocument
        Pre-built PDF with signal distributions.
    peak_equity : float
        Starting peak equity for drawdown scaling.
    kelly_fraction_mult : float
        Kelly multiplier (default 0.5 = half-Kelly).
    probability_levels : list[float] | None
        Target OTM probabilities for strike selection.
    signal_ci_threshold : float
        Maximum 95% CI width for a signal to be usable (default 0.01).
    """
    generate_signal_stats(playground, symbol)
    playground.stats.generate_model()

    strategy = PDFWheelStrategy(
        playground, symbol, logger,
        pdf=pdf,
        peak_equity=peak_equity,
        kelly_fraction_mult=kelly_fraction_mult,
        probability_levels=probability_levels,
        max_open_count=5,
        signal_ci_threshold=signal_ci_threshold,
    )

    # ---- Funnel counters ----
    funnel_ltf_bars = 0           # LTF bars processed
    funnel_signals_detected = 0   # PDF signals detected (compound match found)
    funnel_kelly_zero = 0         # Blocked: Kelly sizing returned 0
    funnel_kelly_ok = 0           # Passed Kelly sizing
    funnel_ladder_empty = 0       # Blocked: empty ladder response
    funnel_ladder_ok = 0          # Passed ladder fetch
    funnel_no_strikes = 0         # Blocked: no OTM strikes / empty allocation
    funnel_strikes_ok = 0         # Had valid strike allocations
    funnel_position_exists = 0    # Blocked: position already exists
    funnel_orders_placed = 0      # Actual orders placed
    funnel_at_max_positions = 0   # Skipped: already at max open positions

    # Track contracts we've already opened to avoid redundant attempts
    opened_contracts: set = set()

    while not strategy.is_complete():
        tick_deltas = playground.flush_new_state_buffer()

        for tick_delta in tick_deltas:
            open_signals, close_signals = strategy.tick(tick_delta)

            # Count LTF bars processed this tick
            new_candles = (
                tick_delta.new_candles if hasattr(tick_delta, "new_candles") else []
            )
            for c in new_candles:
                if hasattr(c, "period") and c.period == playground.ltf_seconds:
                    if hasattr(c, "symbol") and c.symbol == symbol:
                        funnel_ltf_bars += 1

            # ---- Close signals ----
            for signal in close_signals:
                playground.place_order(
                    signal.option_contract.symbol,
                    abs(signal.quantity_to_close),
                    OrderSide.BUY_TO_CLOSE,
                    "option",
                    with_tick=True,
                )
                opened_contracts.discard(signal.option_contract.symbol)
                logger.info(
                    f"Close Signal: {signal.name} at {signal.timestamp}"
                    f" for {signal.option_contract.symbol}"
                )

            # ---- Open signals ----
            for signal in open_signals:
                if isinstance(signal, PDFPutSignal):
                    funnel_signals_detected += 1

                    # Compute position size via Kelly
                    total_contracts = strategy.compute_total_contracts(
                        signal.pdf_entry, signal.price,
                    )
                    if total_contracts <= 0:
                        funnel_kelly_zero += 1
                        logger.debug(
                            f"[funnel] Kelly=0 for {signal.compound_key}"
                            f" @ ${signal.price:.2f}"
                        )
                        continue

                    funnel_kelly_ok += 1

                    # Fetch options ladder (this week + next week)
                    exp_this_week = strategy.find_next_friday(signal.timestamp)
                    exp_next_week = exp_this_week + 7
                    request = GetOptionsLadderRequest(
                        playground_id=playground.id,
                        stock_symbol=signal.symbol,
                        max_no_of_strikes=10,
                        min_distance_between_strikes=1.0,
                        expiration_in_days=[exp_this_week, exp_next_week],
                        max_tick_age_in_minutes=1440,
                    )
                    response = playground.fetch_ladder(request)
                    if not response or not response.contracts:
                        funnel_ladder_empty += 1
                        logger.debug(
                            f"[funnel] Empty ladder for {signal.compound_key}"
                            f" @ ${signal.price:.2f}"
                            f" (exp_days={exp_this_week},{exp_next_week})"
                        )
                        continue

                    funnel_ladder_ok += 1

                    # Select strikes and allocate contracts
                    allocations = strategy.select_put_strikes(
                        signal.pdf_entry, signal.price,
                        response.contracts, total_contracts,
                    )

                    if not allocations:
                        funnel_no_strikes += 1
                        logger.debug(
                            f"[funnel] No OTM strikes for {signal.compound_key}"
                            f" @ ${signal.price:.2f}"
                            f" ({len(response.contracts)} contracts in ladder)"
                        )
                        continue

                    funnel_strikes_ok += 1

                    for alloc in allocations:
                        # Skip contracts we've already opened this run
                        if alloc.contract_symbol in opened_contracts:
                            funnel_position_exists += 1
                            continue

                        existing = playground.account.get_position(alloc.contract_symbol)
                        if existing is not None and existing.quantity != 0:
                            funnel_position_exists += 1
                            opened_contracts.add(alloc.contract_symbol)
                            logger.debug(
                                f"[funnel] Position exists for {alloc.contract_symbol}"
                                f" (qty={existing.quantity}). Skipping."
                            )
                            continue

                        playground.place_order(
                            alloc.contract_symbol,
                            alloc.contracts,
                            OrderSide.SELL_TO_OPEN,
                            "option",
                            attributes={
                                "signal_name": signal.name,
                                "compound_key": signal.compound_key,
                                "stock_price": str(signal.price),
                                "probability": str(alloc.probability),
                            },
                        )
                        opened_contracts.add(alloc.contract_symbol)
                        funnel_orders_placed += 1
                        logger.info(
                            f"Open Signal (PDF Put): {signal.name}"
                            f" → sold {alloc.contracts}x {alloc.contract_symbol}"
                            f" (strike={alloc.strike}, P(OTM)={alloc.probability:.0%})"
                        )

                # --------------------------------------------------
                # Phase 2: sell a covered call (or roll an existing one)
                # --------------------------------------------------
                elif isinstance(signal, (OpenSignalV4, RollSignalV1)):
                    if isinstance(signal, OpenSignalV4):
                        expiration_in_days = strategy.find_next_friday(signal.timestamp)
                    else:  # RollSignalV1
                        expiration_in_days = strategy.find_next_friday(
                            signal.option_contract.expiration_date
                        )

                    if isinstance(signal, RollSignalV1):
                        expiration_days_list = [
                            expiration_in_days,
                            expiration_in_days + 7,
                            expiration_in_days + 14,
                        ]
                    else:
                        expiration_days_list = [expiration_in_days]

                    request = GetOptionsLadderRequest(
                        playground_id=playground.id,
                        stock_symbol=signal.symbol,
                        max_no_of_strikes=5,
                        min_distance_between_strikes=1.0,
                        expiration_in_days=expiration_days_list,
                        max_tick_age_in_minutes=1440,
                    )

                    response = playground.fetch_ladder(request)
                    if not response:
                        continue

                    target_contract = None
                    highest_expected_profit = -1.0

                    for c in response.contracts:
                        if c.type != "call":
                            continue

                        premium_received = (c.bid + c.ask) / 2
                        T = expiration_in_days / 365.0
                        sigma = playground.stats.calculate_local_model_volatility(signal)

                        profit = calculate_expected_profit_binomial_american(
                            S0=signal.price,
                            P2=c.strike,
                            P3=premium_received,
                            sigma=sigma,
                            T=T,
                            r=0.04,
                            N=100,
                        )

                        if isinstance(signal, RollSignalV1):
                            position = playground.account.positions.get(
                                signal.option_contract.symbol
                            )
                            if position is None:
                                logger.warning(
                                    f"No position found for {signal.option_contract.symbol}"
                                    " while processing roll signal."
                                )
                                continue
                            if not signal.name.startswith("ROLL_CALL_LOSS_RATIO"):
                                if premium_received - position.current_price < 0:
                                    continue

                        if profit > highest_expected_profit:
                            highest_expected_profit = profit
                            target_contract = c

                    if target_contract is None:
                        logger.warning(
                            f"No suitable call contract found for {signal.symbol}"
                            f" at price {signal.price}"
                        )
                        continue

                    attributes = {
                        "ev": str(highest_expected_profit),
                        "stock_price": str(signal.price),
                        "signal_name": signal.name,
                    }

                    # Check for existing position BEFORE buying stock
                    if isinstance(signal, OpenSignalV4):
                        existing = playground.account.get_position(
                            target_contract.symbol
                        )
                        if existing is not None and existing.quantity != 0:
                            logger.debug(
                                f"Call position already exists for {target_contract.symbol}"
                                f" (qty={existing.quantity}). Skipping."
                            )
                            continue

                    if isinstance(signal, RollSignalV1):
                        playground.place_order(
                            signal.option_contract.symbol,
                            signal.quantity_to_roll,
                            OrderSide.BUY_TO_CLOSE,
                            "option",
                        )
                        attributes["roll_from"] = signal.option_contract.symbol
                    else:
                        stock_qty = calculate_stock_quantity(
                            playground, signal.symbol, -1
                        )
                        if stock_qty > 0:
                            playground.place_order(
                                signal.symbol,
                                stock_qty,
                                OrderSide.BUY,
                                "equity",
                                signal.price,
                            )

                    playground.place_order(
                        target_contract.symbol,
                        1,
                        OrderSide.SELL_TO_OPEN,
                        "option",
                        attributes=attributes,
                    )
                    funnel_orders_placed += 1
                    logger.info(
                        f"Open Signal (Call): {signal.name} at {signal.timestamp}"
                        f" → sold {target_contract.symbol}"
                    )

        playground.tick(playground.ltf_seconds)
        logger.debug(f"Ticked to {playground.timestamp.isoformat()}")

    # ---- Funnel summary ----
    logger.info("=" * 60)
    logger.info("SIGNAL FUNNEL SUMMARY")
    logger.info("=" * 60)
    logger.info(f"  LTF bars processed:        {funnel_ltf_bars}")
    logger.info(f"  PDF signals detected:       {funnel_signals_detected}")
    logger.info(f"  Blocked by Kelly (=0):      {funnel_kelly_zero}")
    logger.info(f"  Passed Kelly:               {funnel_kelly_ok}")
    logger.info(f"  Blocked by empty ladder:    {funnel_ladder_empty}")
    logger.info(f"  Passed ladder:              {funnel_ladder_ok}")
    logger.info(f"  Blocked by no OTM strikes:  {funnel_no_strikes}")
    logger.info(f"  Had valid allocations:      {funnel_strikes_ok}")
    logger.info(f"  Blocked by existing pos:    {funnel_position_exists}")
    logger.info(f"  Orders placed:              {funnel_orders_placed}")
    logger.info("=" * 60)

    logger.info(f"PDF Wheel Strategy complete — playground id: {playground.id}")
