"""
Wheel Strategy

Phase 1 (SELL_PUTS):  No stock held.  Sell cash-secured OTM puts.
                      If put expires worthless → collect premium, repeat.
                      If assigned → receive 100 shares per contract, move to Phase 2.

Phase 2 (SELL_CALLS): Stock held (from put assignment).  Sell covered calls.
                      If call expires worthless → collect premium, repeat.
                      If stock called away → move back to Phase 1.

The strategy reuses OptionsStrategyBasic (covered-call engine) for Phase 2 and
adds its own put-selling logic for Phase 1.
"""

from datetime import datetime
from dataclasses import dataclass
from typing import List, Tuple, Optional

from dateutil.parser import isoparse
from rpc.playground_pb2 import GetOptionsLadderRequest, OptionLadderContract, Candle

from engine.client import (
    BacktesterPlaygroundClient,
    Repository,
    OrderSide,
)
from engine.types import SignalDecision
from strategies.covered_call import (
    OptionsStrategyBasic,
    OptionContractRepository,
    CloseSignalV2,
    OpenSignalV4,
    RollSignalV1,
    calculate_expected_profit_binomial_american,
    calculate_stock_quantity,
    generate_signal_stats,
)


class WheelPhase:
    SELL_PUTS = "sell_puts"
    SELL_CALLS = "sell_calls"


@dataclass
class PutSellSignal:
    symbol: str
    name: str
    timestamp: datetime
    price: float
    ltf_supertrend_count: int
    ltf_supertrend_value: float
    ltf_supertrend_direction: int
    expected_volatility: float


class WheelStrategy(OptionsStrategyBasic):
    """
    Extends OptionsStrategyBasic to implement the full Wheel Strategy.

    Phase detection is based solely on stock position size:
      - < 100 shares of the underlying  →  Phase 1 (sell puts)
      - >= 100 shares of the underlying →  Phase 2 (sell covered calls)
    """

    def get_current_phase(self) -> str:
        stock_position = self.playground.account.positions.get(self.symbol)
        if stock_position and stock_position.quantity >= 100:
            return WheelPhase.SELL_CALLS
        return WheelPhase.SELL_PUTS

    # ------------------------------------------------------------------
    # Put helpers
    # ------------------------------------------------------------------

    def find_target_put_contract(
        self, current_price: float, contracts
    ) -> Optional[OptionLadderContract]:
        """Return the closest OTM put (strike strictly below current_price)."""
        closest_contract = None
        smallest_diff = float("inf")

        for contract in contracts:
            if contract.type != "put":
                continue
            if contract.strike >= current_price:
                continue  # ITM or ATM – skip
            diff = current_price - contract.strike
            if diff < smallest_diff:
                smallest_diff = diff
                closest_contract = contract

        return closest_contract

    def check_for_put_signal(self, new_candle) -> Optional[PutSellSignal]:
        """
        Generate a sell-put signal using the same supertrend feature vector
        that drives covered-call entries.
        """
        result = self._get_feature_vector(new_candle)
        if result is None:
            return None

        ltf_supertrend_count, ltf_supertrend_value, st_direction = result

        signal = PutSellSignal(
            symbol=self.symbol,
            name="SHORT_PUT_SIGNAL",
            timestamp=isoparse(new_candle.datetime),
            price=new_candle.close,
            ltf_supertrend_count=ltf_supertrend_count,
            ltf_supertrend_value=ltf_supertrend_value,
            ltf_supertrend_direction=st_direction,
            expected_volatility=0.0,
        )

        signal.expected_volatility = self.playground.stats.calculate_local_model_volatility(
            signal
        )
        return signal

    def check_for_early_close_puts(self) -> List[CloseSignalV2]:
        """Close short puts early using the same premium-capture schedule as calls."""
        close_signals = []
        for key, position in self.playground.get_option_positions().items():
            if position.quantity >= 0:
                continue

            contract = self.option_contract_repo.get_contract_details(position.symbol)
            if contract.option_type != "put":
                continue

            days_before_expiration = (
                contract.expiration_date - self.playground.timestamp
            ).days
            premium_captured = position.cost_basis - position.current_price
            premium_captured_pct = (
                premium_captured / position.cost_basis
                if position.cost_basis > 0
                else 0.0
            )

            if self.early_close_schedule(days_before_expiration, premium_captured_pct):
                signal_name = (
                    f"EARLY_CLOSE_PUT_DBE_{days_before_expiration}"
                    f"_PCP_{premium_captured_pct:.2f}"
                )
                close_signals.append(
                    CloseSignalV2(
                        name=signal_name,
                        timestamp=self.playground.timestamp,
                        price=position.current_price,
                        option_contract=contract,
                        quantity_to_close=abs(position.quantity),
                    )
                )

        return close_signals

    # ------------------------------------------------------------------
    # Tick override: dispatch to the appropriate phase
    # ------------------------------------------------------------------

    def tick(self, tick_delta) -> Tuple[List, List[CloseSignalV2]]:
        phase = self.get_current_phase()

        if phase == WheelPhase.SELL_CALLS:
            self.logger.debug("Wheel Phase 2: Selling Covered Calls")
            return super().tick(tick_delta)

        # ------------------------------------------------------------------
        # Phase 1: Sell cash-secured puts
        # ------------------------------------------------------------------
        self.logger.debug("Wheel Phase 1: Selling Cash-Secured Puts")

        open_signals: List[PutSellSignal] = []
        new_candles: List[Candle] = (
            tick_delta.new_candles if hasattr(tick_delta, "new_candles") else []
        )

        for c in new_candles:
            if c.period != self.playground.ltf_seconds:
                continue
            if c.symbol != self.symbol:
                continue

            open_qty = self.playground.get_options_quantity(self.symbol)
            if abs(open_qty) < self.max_open_count:
                signal = self.check_for_put_signal(c.bar)
                if signal:
                    open_signals.append(signal)

            self.append_candle(c.bar)

        close_signals = self.check_for_early_close_puts()
        return open_signals, close_signals

    # ------------------------------------------------------------------ #
    # BaseStrategy interface override
    # ------------------------------------------------------------------ #

    def on_tick(self, tick_deltas) -> None:
        """Process tick deltas for the Wheel Strategy.

        Handles PutSellSignal (Phase 1), OpenSignalV4, and RollSignalV1 (Phase 2).
        """
        for tick_delta in tick_deltas:
            open_signals, close_signals = self.tick(tick_delta)

            # Place close orders (both puts and calls)
            for signal in close_signals:
                self.playground.place_order(
                    signal.option_contract.symbol,
                    abs(signal.quantity_to_close),
                    OrderSide.BUY_TO_CLOSE,
                    'option',
                    with_tick=True,
                )
                self.record_decision(SignalDecision(
                    signal_type="wheel", direction="neutral",
                    decision="place", reason=f"early close: {signal.name}",
                    symbol="", playground_id="",
                ))
                self.logger.info(
                    f"Close Signal: {signal.name} at {signal.timestamp}"
                    f" for {signal.option_contract.symbol}"
                )

            # Place open orders
            for signal in open_signals:
                # Phase 1: sell a cash-secured put
                if isinstance(signal, PutSellSignal):
                    expiration_in_days = self.find_next_friday(signal.timestamp)

                    request = GetOptionsLadderRequest(
                        playground_id=self.playground.id,
                        stock_symbol=signal.symbol,
                        max_no_of_strikes=5,
                        min_distance_between_strikes=1.0,
                        expiration_in_days=[expiration_in_days],
                        max_tick_age_in_minutes=1440,
                    )

                    response = self.playground.fetch_ladder(request)
                    if not response:
                        continue

                    target_contract = self.find_target_put_contract(
                        signal.price, response.contracts,
                    )
                    if target_contract is None:
                        self.record_decision(SignalDecision(
                            signal_type="wheel", direction="short",
                            decision="skip", reason="no suitable put contract found",
                            symbol="", playground_id="",
                        ))
                        self.logger.warning(
                            f"No suitable put contract found for {signal.symbol}"
                            f" at price {signal.price}"
                        )
                        continue

                    existing = self.playground.account.get_position(target_contract.symbol)
                    if existing is not None and existing.quantity != 0:
                        self.record_decision(SignalDecision(
                            signal_type="wheel", direction="short",
                            decision="skip", reason="put position already exists",
                            symbol="", playground_id="",
                        ))
                        self.logger.warning(
                            f"Put position already exists for {target_contract.symbol}"
                            f" (qty={existing.quantity}). Skipping."
                        )
                        continue

                    attributes = {
                        "signal_name": signal.name,
                        "stock_price": str(signal.price),
                    }
                    self.playground.place_order(
                        target_contract.symbol,
                        1,
                        OrderSide.SELL_TO_OPEN,
                        'option',
                        attributes=attributes,
                    )
                    self.record_decision(SignalDecision(
                        signal_type="wheel", direction="short",
                        decision="place", reason=f"sell put: {signal.name}",
                        symbol="", playground_id="",
                    ))
                    self.logger.info(
                        f"Open Signal (Put): {signal.name} at {signal.timestamp}"
                        f" -> sold {target_contract.symbol}"
                    )

                # Phase 2: sell a covered call (or roll an existing one)
                elif isinstance(signal, (OpenSignalV4, RollSignalV1)):
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
                                self.logger.warning(
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
                        self.record_decision(SignalDecision(
                            signal_type="wheel", direction="short",
                            decision="skip", reason="no suitable call contract found",
                            symbol="", playground_id="",
                        ))
                        self.logger.warning(
                            f"No suitable call contract found for {signal.symbol}"
                            f" at price {signal.price}"
                        )
                        continue

                    attributes = {
                        "ev": str(highest_expected_profit),
                        "stock_price": str(signal.price),
                        "signal_name": signal.name,
                    }

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

                    if isinstance(signal, OpenSignalV4):
                        existing = self.playground.account.get_position(
                            target_contract.symbol
                        )
                        if existing is not None and existing.quantity != 0:
                            self.logger.warning(
                                f"Call position already exists for {target_contract.symbol}"
                                f" (qty={existing.quantity}). Skipping."
                            )
                            continue

                    self.playground.place_order(
                        target_contract.symbol,
                        1,
                        OrderSide.SELL_TO_OPEN,
                        'option',
                        attributes=attributes,
                    )
                    self.record_decision(SignalDecision(
                        signal_type="wheel", direction="short",
                        decision="place", reason=f"sell call: {signal.name}",
                        symbol="", playground_id="",
                    ))
                    self.logger.info(
                        f"Open Signal (Call): {signal.name} at {signal.timestamp}"
                        f" -> sold {target_contract.symbol}"
                    )


# ----------------------------------------------------------------------
# Strategy runner
# ----------------------------------------------------------------------

def run_wheel_strategy(
    playground: BacktesterPlaygroundClient,
    symbol: str,
    logger,
    twirp_host: str,
) -> None:
    """
    Main loop for the Wheel Strategy.  Mirrors run_options_strategy but handles
    both PutSellSignal (Phase 1) and OpenSignalV4 / RollSignalV1 (Phase 2).
    """
    generate_signal_stats(playground, symbol)
    playground.stats.generate_model()

    max_open_count = 3
    strategy = WheelStrategy(playground, symbol, logger, max_open_count=max_open_count)

    while not strategy.is_complete():
        tick_deltas = playground.flush_new_state_buffer()

        for tick_delta in tick_deltas:
            open_signals, close_signals = strategy.tick(tick_delta)

            # ---- Close signals (both puts and calls) ----
            for signal in close_signals:
                playground.place_order(
                    signal.option_contract.symbol,
                    abs(signal.quantity_to_close),
                    OrderSide.BUY_TO_CLOSE,
                    "option",
                    with_tick=True,
                )
                logger.info(
                    f"Close Signal: {signal.name} at {signal.timestamp}"
                    f" for {signal.option_contract.symbol}"
                )

            # ---- Open signals ----
            for signal in open_signals:

                # --------------------------------------------------
                # Phase 1: sell a cash-secured put
                # --------------------------------------------------
                if isinstance(signal, PutSellSignal):
                    expiration_in_days = strategy.find_next_friday(signal.timestamp)

                    request = GetOptionsLadderRequest(
                        playground_id=playground.id,
                        stock_symbol=signal.symbol,
                        max_no_of_strikes=5,
                        min_distance_between_strikes=1.0,
                        expiration_in_days=[expiration_in_days],
                        max_tick_age_in_minutes=1440,
                    )

                    response = playground.fetch_ladder(request)
                    if not response:
                        continue

                    target_contract = strategy.find_target_put_contract(
                        signal.price, response.contracts
                    )
                    if target_contract is None:
                        logger.warning(
                            f"No suitable put contract found for {signal.symbol}"
                            f" at price {signal.price}"
                        )
                        continue

                    existing = playground.account.get_position(target_contract.symbol)
                    if existing is not None and existing.quantity != 0:
                        logger.warning(
                            f"Put position already exists for {target_contract.symbol}"
                            f" (qty={existing.quantity}). Skipping."
                        )
                        continue

                    attributes = {
                        "signal_name": signal.name,
                        "stock_price": str(signal.price),
                    }
                    playground.place_order(
                        target_contract.symbol,
                        1,
                        OrderSide.SELL_TO_OPEN,
                        "option",
                        attributes=attributes,
                    )
                    logger.info(
                        f"Open Signal (Put): {signal.name} at {signal.timestamp}"
                        f" → sold {target_contract.symbol}"
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

                    if isinstance(signal, RollSignalV1):
                        playground.place_order(
                            signal.option_contract.symbol,
                            signal.quantity_to_roll,
                            OrderSide.BUY_TO_CLOSE,
                            "option",
                        )
                        attributes["roll_from"] = signal.option_contract.symbol
                    else:
                        # Buy stock to cover the call if needed
                        from strategies.covered_call import calculate_stock_quantity

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

                    if isinstance(signal, OpenSignalV4):
                        existing = playground.account.get_position(
                            target_contract.symbol
                        )
                        if existing is not None and existing.quantity != 0:
                            logger.warning(
                                f"Call position already exists for {target_contract.symbol}"
                                f" (qty={existing.quantity}). Skipping."
                            )
                            continue

                    playground.place_order(
                        target_contract.symbol,
                        1,
                        OrderSide.SELL_TO_OPEN,
                        "option",
                        attributes=attributes,
                    )
                    logger.info(
                        f"Open Signal (Call): {signal.name} at {signal.timestamp}"
                        f" → sold {target_contract.symbol}"
                    )

        playground.tick(playground.ltf_seconds)
        logger.info(f"Ticked playground to {playground.timestamp.isoformat()}")

    logger.info(f"Done — playground id: {playground.id}")
