"""
Covered Call Strategy V2 -- datasource-consuming variant.

Identical to OptionsStrategyBasic (V1) except check_for_new_signal() delegates
signal detection to datasources.covered_call_signals.produce_open_signals()
instead of computing the feature vector inline. This follows the TradeSignal
framework datasource pattern established in Phase 21 (MeanReversionV2).

Supertrend-based covered call strategy:
- Detects supertrend direction changes on LTF candles
- Sells covered calls when signal fires (SHORT_CALL_SIGNAL)
- Manages roll signals and early close signals
- Selects option contracts via expected profit (binomial American model)

The V2 class is a full copy of V1 behavior. Only the signal DETECTION path
changes (delegated to the datasource). All other logic -- order placement,
close/roll, option contract selection -- is identical.
"""

from __future__ import annotations

from datetime import datetime, timedelta
from typing import List, Tuple, Optional

import pandas as pd
from dateutil.parser import isoparse
from loguru import logger

from engine.client import (
    BacktesterPlaygroundClient,
    Repository,
    RepositorySource,
    OrderSide,
)
from engine.types import OpenSignalV3, OpenSignalName, SignalDecision
from rpc.playground_pb2 import GetOptionsLadderRequest, OptionLadderContract, Candle
from strategies.base_strategy import BaseStrategy
from datasources.covered_call_signals import produce_open_signals

# Import shared types from V1 to avoid duplication
from deprecated.covered_call import (
    OptionContract,
    OptionContractRepository,
    CloseSignalV2,
    OpenSignalV4,
    RollSignalV1,
    calculate_expected_profit_binomial_american,
    calculate_stock_quantity,
    generate_signal_stats,
)
# Import BaseOpenStrategyV2 for append_candle and candle state management
from deprecated.base_open_strategy_v2 import BaseOpenStrategyV2


class OptionsStrategyBasicV2(BaseOpenStrategyV2, BaseStrategy):
    """Covered call strategy V2 -- delegates signal detection to datasource.

    Full copy of OptionsStrategyBasic (V1) logic. The only change is
    check_for_new_signal() which calls produce_open_signals() from the
    covered_call_signals datasource instead of computing inline.
    """

    label = "covered_call"

    @classmethod
    def get_repositories(cls, symbol: str, start_date: datetime, end_date: datetime) -> List[Repository]:
        ltf_repo_daily = Repository(
            symbol=symbol,
            timespan_multiplier=1,
            timespan_unit='hour',
            indicators=["supertrend", "atr", "doji", "hammer"],
            history_in_days=365
        )

        htf_repo_daily = Repository(
            symbol=symbol,
            timespan_multiplier=1,
            timespan_unit='day',
            indicators=["supertrend", "atr", "doji", "hammer"],
            history_in_days=365
        )

        return [ltf_repo_daily, htf_repo_daily]

    def __init__(self, playground, symbol: str, logger=None, max_open_count: int = 3, sl_buffer=0.0, tp_buffer=0.0):
        sl_shift = 0.0
        tp_shift = 0.0

        BaseOpenStrategyV2.__init__(self, playground, symbol, sl_shift, tp_shift, sl_buffer, tp_buffer)
        BaseStrategy.__init__(self, playground, symbol, logger)

        self.logger = logger if logger else globals()['logger']
        if hasattr(self.logger, 'bind'):
            self.logger = self.logger.bind(symbol=symbol)
        self.symbol = symbol
        self.max_open_count = max_open_count
        self.use_htf_data = True
        self.candles = []
        self.option_contract_repo = OptionContractRepository()

    def _get_intrinsic_value(self, option_type: str, strike_price: float, underlying_price: float) -> float:
        if option_type == 'call':
            intrinsic_value = max(0.0, underlying_price - strike_price)
        elif option_type == 'put':
            intrinsic_value = max(0.0, strike_price - underlying_price)
        else:
            raise ValueError("Invalid option type. Must be 'call' or 'put'.")

        return intrinsic_value

    def _get_feature_vector(self, new_candle: pd.DataFrame) -> Optional[Tuple[int, float, int]]:
        """Extract the supertrend feature vector from candle data.

        Looks back through candles_ltf to find the most recent supertrend
        direction change, returning the count of bars since change, the
        supertrend value, and the current direction.
        """
        st_direction = new_candle.superD_50_3
        previous_supertrend_count = self.candles_ltf_idx
        for i in range(self.candles_ltf_idx - 1, 0, -1):
            previous_direction = self.candles_ltf.iloc[i].superD_50_3
            if previous_direction != st_direction:
                ltf_supertrend_count = abs(previous_supertrend_count - i)
                ltf_supertrend_value = new_candle.superT_50_3
                previous_supertrend_count = i

                return ltf_supertrend_count, ltf_supertrend_value, st_direction

        return None

    def check_for_new_signal(self, new_candle: pd.DataFrame):
        """V2: Delegate signal detection to the covered_call_signals datasource.

        Uses produce_open_signals() with a feature_vector_fn callable that
        wraps self._get_feature_vector() for stateful lookback.
        """
        def feature_vector_fn(candle):
            return self._get_feature_vector(candle)

        signals = produce_open_signals(
            feature_vector_fn=feature_vector_fn,
            candle=new_candle,
            symbol=self.symbol,
        )

        if not signals:
            return None

        sig = signals[0]
        signal = OpenSignalV4(
            symbol=sig["symbol"],
            name=sig["signal_name"],
            timestamp=isoparse(sig["timestamp"]),
            price=sig["price"],
            ltf_supertrend_count=sig["ltf_supertrend_count"],
            ltf_supertrend_value=sig["ltf_supertrend_value"],
            ltf_supertrend_direction=sig["ltf_supertrend_direction"],
            expected_volatility=sig["expected_volatility"],
        )

        signal.expected_volatility = self.playground.stats.calculate_local_model_volatility(signal)

        return signal

    # ------------------------------------------------------------------ #
    # Roll and close signals (identical to V1)
    # ------------------------------------------------------------------ #

    def check_for_roll_signal(self, option_prices: dict) -> List[RollSignalV1]:
        previous_candle = self.candles_ltf.iloc[self.candles_ltf_idx - 1]

        if previous_candle is None:
            logger.warning("Underlying price is None, cannot check for roll signals.")
            return []

        signals = []
        positions = self.playground.get_option_positions()
        last_price = previous_candle.close
        for key in positions:
            position = positions[key]
            if position.quantity >= 0:
                continue  # Only consider short positions

            contract = self.option_contract_repo.get_contract_details(position.symbol)
            if contract.option_type != 'call':
                raise Exception("Only call options are supported for rolling in this strategy.")

            if last_price < contract.strike_price:
                continue  # Out of the money, no roll signal

            option_price = option_prices.get(position.symbol, None)
            if option_price is None:
                logger.trace(f"No option price available for {position.symbol}, cannot check for roll signal.")
                continue

            hours_to_expiration = (contract.expiration_date - self.playground.timestamp).total_seconds() / 3600.0
            intrinsic_value = self._get_intrinsic_value(contract.option_type, contract.strike_price, last_price)
            intrinsic_potential = intrinsic_value / option_price if option_price > 0 else 0.0

            extrinsic_value_ratio = hours_to_expiration / intrinsic_potential if intrinsic_potential > 0 else float('inf')
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
                logger.warning("Feature vector is None, cannot check for roll signal.")
                continue

            ltf_supertrend_count, ltf_supertrend_value, st_direction = result

            signal = RollSignalV1(
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
            signals.append(signal)

        return signals

    def get_max_per_trade_risk_percentage(self):
        return self.max_per_trade_risk_percentage

    def get_sl_shift(self):
        return self.sl_shift

    def get_tp_shift(self):
        return self.tp_shift

    def get_sl_buffer(self):
        return self.sl_buffer

    def get_tp_buffer(self):
        return self.tp_buffer

    def find_supertrend_start(self, df: pd.DataFrame) -> int:
        current_direction = df.iloc[-1]['superD_50_3']
        for i in range(-1, -1 * len(df), -1):
            previous_direction = df.iloc[i]['superD_50_3']
            if previous_direction != current_direction:
                return i

        return -1

    def find_next_friday(self, current_date: datetime) -> int:
        days_ahead = 4 - current_date.weekday()  # Friday is 4
        if days_ahead <= 0:
            days_ahead += 7
        return days_ahead

    def find_target_option_contract(self, current_price, contracts) -> OptionLadderContract:
        closest_contract = None
        smallest_diff = float('inf')

        for contract in contracts:
            if contract.type != 'call':
                continue

            strike_price = contract.strike
            diff = abs(strike_price - current_price)
            if diff < smallest_diff:
                smallest_diff = diff
                closest_contract = contract

        return closest_contract

    def early_close_schedule(self, days_before_expiration: int, premium_captured_percentage: float):
        if days_before_expiration >= 5 and premium_captured_percentage >= 0.50:
            return True
        elif days_before_expiration >= 4 and premium_captured_percentage >= 0.60:
            return True
        elif days_before_expiration >= 3 and premium_captured_percentage >= 0.70:
            return True
        elif days_before_expiration >= 2 and premium_captured_percentage >= 0.80:
            return True
        else:
            return False

    def check_for_early_close_signals(self, positions: dict) -> List[CloseSignalV2]:
        close_signals = []
        for key in positions:
            position = positions[key]
            if position.quantity >= 0:
                continue  # Only consider short positions

            contract = self.option_contract_repo.get_contract_details(position.symbol)
            days_before_expiration = (contract.expiration_date - self.playground.timestamp).days

            premium_captured = position.cost_basis - position.current_price
            premium_captured_percentage = premium_captured / position.cost_basis if position.cost_basis > 0 else 0.0

            if self.early_close_schedule(days_before_expiration, premium_captured_percentage):
                signal_name = f"EARLY_CLOSE_DBE_{days_before_expiration}_PCP_{premium_captured_percentage:.2f}"
                close_signal = CloseSignalV2(
                    name=signal_name,
                    timestamp=self.playground.timestamp,
                    price=position.current_price,
                    option_contract=contract,
                    quantity_to_close=abs(position.quantity),
                )
                close_signals.append(close_signal)

        return close_signals

    # ------------------------------------------------------------------ #
    # Tick (identical to V1 -- only check_for_new_signal differs above)
    # ------------------------------------------------------------------ #

    def tick(self, tick_delta) -> Tuple[List[OpenSignalV4], List[CloseSignalV2]]:
        # Check for assigned options
        position = self.playground.account.positions.get(self.symbol, None)
        if position is not None:
            qty = position.quantity
            if qty < 0:
                requested_price = self.playground.get_current_candle(self.symbol, self.playground.ltf_seconds).close

                # Buy the underlying stock at market price to close the assigned options
                self.playground.place_order(
                    self.symbol,
                    abs(qty),
                    OrderSide.BUY_TO_COVER,
                    'equity',
                    requested_price,
                    with_tick=True
                )

                self.logger.info(f"Closed assigned options by buying {abs(qty)} shares of {self.symbol} at market price.")

            elif qty > 0:
                options_qty = self.playground.get_options_quantity(self.symbol)
                if options_qty < 0:
                    exposure_qty = qty + (options_qty * 100)

                    if exposure_qty > 0:
                        requested_price = self.playground.get_current_candle(self.symbol, self.playground.ltf_seconds).close

                        # Sell the underlying stock to reduce exposure
                        self.playground.place_order(
                            self.symbol,
                            abs(exposure_qty),
                            OrderSide.SELL,
                            'equity',
                            requested_price,
                            with_tick=True
                        )

                        self.logger.info(f"Reduced stock exposure by selling {abs(qty)} shares of {self.symbol} at market price.")

        # Check for new open signals
        open_signals = []
        new_candles: List[Candle] = tick_delta.new_candles if hasattr(tick_delta, 'new_candles') else []
        option_prices = {}
        for c in new_candles:
            self.logger.trace(f"Processing candle - {c.period} @ {c.bar.datetime} - {c.bar.close}")

            if not c.period == self.playground.ltf_seconds:
                continue

            if not c.symbol == self.symbol:
                option_prices[c.symbol] = c.bar.close
                continue

            open_qty = self.playground.get_options_quantity(self.symbol)
            if abs(open_qty) < self.max_open_count:
                open_signal = self.check_for_new_signal(c.bar)
                if open_signal:
                    open_signals.append(open_signal)

            self.append_candle(c.bar)

        roll_signals = self.check_for_roll_signal(option_prices)
        for signal in roll_signals:
            open_signals.append(signal)

        close_signals = self.check_for_early_close_signals(positions=self.playground.get_option_positions())

        return open_signals, close_signals

    # ------------------------------------------------------------------ #
    # BaseStrategy interface methods
    # ------------------------------------------------------------------ #

    def on_tick(self, tick_deltas) -> None:
        """Process tick deltas: generate signals and place orders internally.

        Wraps the existing tick() method and the order-placement logic.
        Identical to V1 on_tick().
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
                    signal_type="covered_call", direction="neutral",
                    decision="place", reason=f"early close: {signal.name}",
                    symbol="", playground_id="",
                ))
                self.logger.info(
                    f"Close Signal: {signal.name} at {signal.timestamp}"
                    f" for {signal.option_contract.symbol}"
                )

            # Place open orders
            for signal in open_signals:
                if isinstance(signal, OpenSignalV4):
                    expiration_in_days = self.find_next_friday(signal.timestamp)
                elif isinstance(signal, RollSignalV1):
                    expiration_in_days = self.find_next_friday(signal.option_contract.expiration_date)
                else:
                    continue

                if isinstance(signal, RollSignalV1):
                    request = GetOptionsLadderRequest(
                        playground_id=self.playground.id,
                        stock_symbol=signal.symbol,
                        max_no_of_strikes=5,
                        min_distance_between_strikes=1.0,
                        expiration_in_days=[expiration_in_days, expiration_in_days + 7, expiration_in_days + 14],
                        max_tick_age_in_minutes=1440,
                    )
                else:
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

                target_contract = None
                highest_expected_profit = -1
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
                        position = self.playground.account.positions.get(signal.option_contract.symbol)
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
                        signal_type="covered_call", direction="short",
                        decision="skip", reason="no suitable call contract found",
                        symbol="", playground_id="",
                    ))
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
                    existing = self.playground.account.get_position(target_contract.symbol)
                    if existing is not None and existing.quantity != 0:
                        continue

                self.playground.place_order(
                    target_contract.symbol,
                    1,
                    OrderSide.SELL_TO_OPEN,
                    'option',
                    attributes=attributes,
                )
                self.record_decision(SignalDecision(
                    signal_type="covered_call", direction="short",
                    decision="place", reason=f"sell call: {signal.name}",
                    symbol="", playground_id="",
                ))
                self.logger.info(
                    f"Open Signal: {signal.name} at {signal.timestamp}"
                    f" -> sold {target_contract.symbol}"
                )

    def get_next_tick_seconds(self) -> int:
        """Covered call always ticks at LTF interval."""
        return self.playground.ltf_seconds

    def should_fetch_account(self) -> bool:
        """Always fetch account state for covered call strategy."""
        return True
