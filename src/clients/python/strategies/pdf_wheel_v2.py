"""
PDF-Guided Wheel Strategy V2 -- datasource-consuming variant.

Identical to PDFWheelStrategy (V1) except check_for_pdf_put_signals() delegates
compound signal detection to datasources.pdf_wheel_signals.produce_signals()
instead of computing inline. Extends WheelStrategyV2 (not WheelStrategy V1)
per D-06.

Key differences from vanilla WheelStrategyV2:
  - Uses 15-min LTF candles (not 1-hour)
  - Detects compound signals (candlestick + indicator combos) via pdf_wheel_signals datasource
  - Sells puts at multiple strike levels (20%-50% probability) simultaneously
  - Sizes positions via Kelly criterion with drawdown scaling
  - PDF lookup replaces supertrend-only signal generation

Phase 1 (SELL_PUTS):  No stock held.  Detect compound signals via datasource,
                      look up PDF, sell puts at multiple OTM strikes based on
                      PDF percentiles.
Phase 2 (SELL_CALLS): Stock held.  Sell covered calls (delegates to parent
                      WheelStrategyV2 -> OptionsStrategyBasicV2).
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

from engine.client import (
    BacktesterPlaygroundClient,
    Repository,
    OrderSide,
)
from engine.types import SignalDecision
from lib.pdf_builder import detect_atomic_signals_on_bar, _get, _get_dt
from lib.pdf_types import PDFDocument, SignalPDF, CompoundSignal
from lib.risk_management import (
    kelly_fraction,
    adjusted_kelly,
    max_contracts,
    allocate_contracts,
)
from strategies.wheel_v2 import WheelStrategyV2
from strategies.wheel import WheelPhase
from strategies.covered_call import (
    CloseSignalV2,
    OpenSignalV4,
    RollSignalV1,
    calculate_expected_profit_binomial_american,
    calculate_stock_quantity,
    generate_signal_stats,
)
from datasources.pdf_wheel_signals import produce_signals

# Import PDF-specific types from V1 to avoid duplication
from strategies.pdf_wheel import PDFPutSignal, StrikeAllocation


# ------------------------------------------------------------------ #
# PDF-Guided Wheel Strategy V2
# ------------------------------------------------------------------ #

class PDFWheelStrategyV2(WheelStrategyV2):
    """
    Wheel strategy V2 that uses empirical PDF distributions to select
    put strikes and size positions.

    Extends WheelStrategyV2 (not WheelStrategy V1) per D-06.
    Delegates compound signal detection to pdf_wheel_signals datasource.

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
        more signals through -- Kelly sizing limits exposure on
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

        # Track daily context signals (date_str -> [signal_names])
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
            "sma_50", "sma_100", "sma_200",
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
    # Signal detection on live bars (V2: delegates to datasource)
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
          - percentile key = str(int((1 - P) * 100))  -> "80", "70", "60", "50"
          - price_drop = pdf_entry.horizons[horizon].percentiles[pct_key]
          - target_strike = current_price * (1 + price_drop)
          - Find closest available OTM put from options ladder
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

        # Margin per contract: strike x 100
        margin = current_price * 100
        total = max_contracts(adj_f, current_equity, margin)

        self.logger.debug(
            f"[kelly] p={win_prob:.2f} b={win_loss_ratio:.2f} f*={f_star:.4f}"
            f" adj_f={adj_f:.4f} margin=${margin:.0f} -> {total} contracts"
        )

        # If Kelly is positive (edge exists) but margin floors to 0,
        # allow at least 1 contract so we don't miss every signal
        if total == 0 and f_star > 0:
            total = 1
            self.logger.debug(
                f"[kelly] Edge exists (f*={f_star:.4f}) -- minimum 1 contract"
            )

        # Update peak equity
        if current_equity > self.peak_equity:
            self.peak_equity = current_equity

        return total

    # ------------------------------------------------------------------ #
    # Override: Phase 1 tick with PDF-guided put signals (V2: datasource)
    # ------------------------------------------------------------------ #

    def check_for_pdf_put_signals(self, bar_dict: dict) -> Optional[PDFPutSignal]:
        """
        V2: Delegate compound signal detection to pdf_wheel_signals datasource.

        Uses produce_signals() which calls detect_atomic_signals_on_bar(),
        merges with daily context, and looks up the PDF for a match.
        """
        signals = produce_signals(
            bar_dict=bar_dict,
            prev_bar=self._prev_ltf_bar,
            pdf=self.pdf,
            daily_signals=self._daily_signals,
            ci_threshold=self.signal_ci_threshold,
        )

        if not signals:
            return None

        sig = signals[0]
        return PDFPutSignal(
            symbol=self.symbol,
            name=sig["signal_name"],
            timestamp=sig["timestamp"],
            price=sig["price"],
            compound_key=sig["compound_key"],
            pdf_entry=sig["pdf_entry"],
        )

    def tick(self, tick_delta) -> Tuple[List, List[CloseSignalV2]]:
        """
        Override tick to use PDF-guided compound signal detection
        for Phase 1 (put selling).  Phase 2 delegates to parent.
        """
        phase = self.get_current_phase()

        if phase == WheelPhase.SELL_CALLS:
            self.logger.debug("PDF Wheel Phase 2: Selling Covered Calls")
            return super().tick(tick_delta)  # WheelStrategyV2.tick handles Phase 2

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
    # BaseStrategy interface override
    # ------------------------------------------------------------------ #

    def on_tick(self, tick_deltas) -> None:
        """Process tick deltas for the PDF-Guided Wheel Strategy V2.

        Handles PDFPutSignal (Phase 1 with Kelly sizing and PDF-guided strikes),
        OpenSignalV4, and RollSignalV1 (Phase 2 via parent).
        """
        for tick_delta in tick_deltas:
            open_signals, close_signals = self.tick(tick_delta)

            # Place close orders
            for signal in close_signals:
                self.playground.place_order(
                    signal.option_contract.symbol,
                    abs(signal.quantity_to_close),
                    OrderSide.BUY_TO_CLOSE,
                    'option',
                    with_tick=True,
                )
                self.record_decision(SignalDecision(
                    signal_type="pdf_wheel", direction="neutral",
                    decision="place", reason=f"early close: {signal.name}",
                    symbol="", playground_id="",
                ))
                self.logger.info(
                    f"Close Signal: {signal.name} at {signal.timestamp}"
                    f" for {signal.option_contract.symbol}"
                )

            # Place open orders
            for signal in open_signals:
                if isinstance(signal, PDFPutSignal):
                    # Compute position size via Kelly
                    total_contracts = self.compute_total_contracts(
                        signal.pdf_entry, signal.price,
                    )
                    if total_contracts <= 0:
                        self.record_decision(SignalDecision(
                            signal_type="pdf_wheel", direction="short",
                            decision="skip", reason="kelly sizing yielded zero contracts",
                            symbol="", playground_id="",
                        ))
                        continue

                    # Fetch options ladder
                    exp_this_week = self.find_next_friday(signal.timestamp)
                    exp_next_week = exp_this_week + 7
                    request = GetOptionsLadderRequest(
                        playground_id=self.playground.id,
                        stock_symbol=signal.symbol,
                        max_no_of_strikes=10,
                        min_distance_between_strikes=1.0,
                        expiration_in_days=[exp_this_week, exp_next_week],
                        max_tick_age_in_minutes=1440,
                    )
                    response = self.playground.fetch_ladder(request)
                    if not response or not response.contracts:
                        continue

                    # Select strikes and allocate
                    allocations = self.select_put_strikes(
                        signal.pdf_entry, signal.price,
                        response.contracts, total_contracts,
                    )
                    if not allocations:
                        continue

                    for alloc in allocations:
                        existing = self.playground.account.get_position(alloc.contract_symbol)
                        if existing is not None and existing.quantity != 0:
                            continue

                        self.playground.place_order(
                            alloc.contract_symbol,
                            alloc.contracts,
                            OrderSide.SELL_TO_OPEN,
                            'option',
                            attributes={
                                "signal_name": signal.name,
                                "compound_key": signal.compound_key,
                                "stock_price": str(signal.price),
                                "probability": str(alloc.probability),
                            },
                        )
                        self.record_decision(SignalDecision(
                            signal_type="pdf_wheel", direction="short",
                            decision="place", reason=f"sell put: {signal.name}",
                            symbol="", playground_id="",
                        ))
                        self.logger.info(
                            f"Open Signal (PDF Put): {signal.name}"
                            f" -> sold {alloc.contracts}x {alloc.contract_symbol}"
                            f" (strike={alloc.strike}, P(OTM)={alloc.probability:.0%})"
                        )

                elif isinstance(signal, (OpenSignalV4, RollSignalV1)):
                    # Phase 2: delegate to parent's on_tick logic for call signals
                    # We handle this inline to avoid double-processing tick_deltas
                    if isinstance(signal, OpenSignalV4):
                        expiration_in_days = self.find_next_friday(signal.timestamp)
                    else:
                        expiration_in_days = self.find_next_friday(
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
                        playground_id=self.playground.id,
                        stock_symbol=signal.symbol,
                        max_no_of_strikes=5,
                        min_distance_between_strikes=1.0,
                        expiration_in_days=expiration_days_list,
                        max_tick_age_in_minutes=1440,
                    )

                    response = self.playground.fetch_ladder(request)
                    if not response:
                        continue

                    target_contract = None
                    highest_expected_profit = -1.0

                    for c in response.contracts:
                        if c.type != 'call':
                            continue
                        premium_received = (c.bid + c.ask) / 2
                        T = expiration_in_days / 365.0
                        sigma = self.playground.stats.calculate_local_model_volatility(signal)
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
                            position = self.playground.account.positions.get(
                                signal.option_contract.symbol
                            )
                            if position is None:
                                continue
                            if not signal.name.startswith("ROLL_CALL_LOSS_RATIO"):
                                if premium_received - position.current_price < 0:
                                    continue

                        if profit > highest_expected_profit:
                            highest_expected_profit = profit
                            target_contract = c

                    if target_contract is None:
                        self.record_decision(SignalDecision(
                            signal_type="pdf_wheel", direction="short",
                            decision="skip", reason="no suitable call contract found",
                            symbol="", playground_id="",
                        ))
                        continue

                    attributes = {
                        "ev": str(highest_expected_profit),
                        "stock_price": str(signal.price),
                        "signal_name": signal.name,
                    }

                    if isinstance(signal, OpenSignalV4):
                        existing = self.playground.account.get_position(
                            target_contract.symbol
                        )
                        if existing is not None and existing.quantity != 0:
                            continue

                    if isinstance(signal, RollSignalV1):
                        self.playground.place_order(
                            signal.option_contract.symbol,
                            signal.quantity_to_roll,
                            OrderSide.BUY_TO_CLOSE,
                            'option',
                        )
                        attributes["roll_from"] = signal.option_contract.symbol
                    else:
                        stock_qty = calculate_stock_quantity(
                            self.playground, signal.symbol, -1,
                        )
                        if stock_qty > 0:
                            self.playground.place_order(
                                signal.symbol,
                                stock_qty,
                                OrderSide.BUY,
                                'equity',
                                signal.price,
                            )

                    self.playground.place_order(
                        target_contract.symbol,
                        1,
                        OrderSide.SELL_TO_OPEN,
                        'option',
                        attributes=attributes,
                    )
                    self.record_decision(SignalDecision(
                        signal_type="pdf_wheel", direction="short",
                        decision="place", reason=f"sell call: {signal.name}",
                        symbol="", playground_id="",
                    ))
                    self.logger.info(
                        f"Open Signal (Call): {signal.name} at {signal.timestamp}"
                        f" -> sold {target_contract.symbol}"
                    )
