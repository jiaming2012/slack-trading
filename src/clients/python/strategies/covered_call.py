from datetime import datetime, time, timedelta, date
from dateutil import parser, tz
from random import random, randint
from typing import List, Tuple, Optional
from loguru import logger
from dateutil.parser import isoparse
from dataclasses import dataclass
import math
import numpy as np
from zoneinfo import ZoneInfo
import re
import matplotlib.pyplot as plt
import seaborn as sns
import statsmodels.api as sm
from scipy.stats import ks_2samp

import pandas as pd
from engine.client import BacktesterPlaygroundClient, Repository, RepositorySource, CreatePolygonPlaygroundRequest, PlaygroundEnvironment, OrderSide
from rpc.playground_pb2 import GetOptionsLadderRequest, OptionLadderContract
from deprecated.base_open_strategy_v2 import BaseOpenStrategyV2
from strategies.base_strategy import BaseStrategy
from engine.types import OpenSignalV3, OpenSignalName, SignalDecision
from rpc.playground_pb2 import Candle

import warnings
warnings.filterwarnings(
    "ignore",
    message="use_inf_as_na option is deprecated",
    category=FutureWarning
)

# v7 adds early close signals based on days to expiration and premium captured percentage
# Adds analysis and visualization of log returns distribution

@dataclass
class OptionContract:
    symbol: str
    underlying_symbol: str
    expiration_date: datetime
    option_type: str
    strike_price: float
    
class OptionContractRepository:
    def __init__(self):
        self.cache = {}
        
    def get_contract_details(self, option_symbol: str) -> OptionContract:
        if option_symbol in self.cache:
            return self.cache[option_symbol]
        
        underlying_symbol, expiration_date, option_type, strike_price = self._parse_option_symbol(option_symbol)
        contract = OptionContract(
            symbol=option_symbol,
            underlying_symbol=underlying_symbol,
            expiration_date=expiration_date,
            option_type=option_type,
            strike_price=strike_price
        )
        self.cache[option_symbol] = contract
        return contract
        
    def _parse_option_symbol(self, option_symbol: str) -> Tuple[str, datetime, str, float]:
        """
        Parse an option symbol in the format: SYMBOL[YY]MMDD[C/P]XXXXXXXX
        
        Example: AMZN251205C00235000
        - AMZN: underlying symbol
        - 251205: expiration date (Dec 5, 2025)  
        - C: call option (P for put)
        - 00235000: strike price (235.000)
        
        Args:
            option_symbol (str): The option symbol to parse
            
        Returns:
            Tuple[str, datetime, str, float]: (underlying_symbol, expiration_date, option_type, strike_price)
            
        Raises:
            ValueError: If the symbol format is invalid
        """
        # Regex pattern to match option symbol format
        # Group 1: Underlying symbol (letters)
        # Group 2: Expiration date (YYMMDD)
        # Group 3: Option type (C or P)
        # Group 4: Strike price (8 digits with implied 3 decimal places)
        pattern = r'^([A-Z]+)(\d{6})([CP])(\d{8})$'
        
        if option_symbol.startswith('O:'):
            option_symbol = option_symbol[2:]
        
        match = re.match(pattern, option_symbol.upper())
        if not match:
            raise ValueError(f"Invalid option symbol format: {option_symbol}")
        
        underlying_symbol, exp_date_str, option_type_char, strike_str = match.groups()
        
        # Parse expiration date (YYMMDD format)
        year = int(exp_date_str[:2])
        month = int(exp_date_str[2:4])
        day = int(exp_date_str[4:6])
        
        # Convert 2-digit year to 4-digit (assume 20xx for now)
        full_year = 2000 + year if year >= 0 else 1900 + year
        
        try:
            expiration_date = datetime(full_year, month, day, 16, 0, 0, tzinfo=ZoneInfo('America/New_York'))
        except ValueError:
            raise ValueError(f"Invalid expiration date in symbol: {exp_date_str}")
        
        # Convert option type character to full word
        option_type = 'call' if option_type_char == 'C' else 'put'
        
        # Parse strike price (8 digits with implied 3 decimal places)
        # Example: 00235000 = 235.000
        strike_price = float(strike_str) / 1000.0
        
        return underlying_symbol, expiration_date, option_type, strike_price

@dataclass
class CloseSignalV2:
    name: str
    timestamp: datetime
    price: float
    option_contract: OptionContract
    quantity_to_close: float
    
@dataclass
class OpenSignalV4:
    symbol: str
    name: str
    timestamp: datetime
    price: float
    ltf_supertrend_count: int
    ltf_supertrend_value: float
    ltf_supertrend_direction: int
    expected_volatility: float
    
@dataclass
class RollSignalV1:
    symbol: str
    name: str
    timestamp: datetime
    price: float
    option_price: float
    intrinsic_value: float
    days_to_expiration: float
    option_contract: OptionContract
    quantity_to_roll: float
    ltf_supertrend_count: int
    ltf_supertrend_value: float
    ltf_supertrend_direction: int
    
def calculate_expected_profit_binomial_american(S0, P2, P3, sigma, T, r, N=100):
        """
        Calculates the expected profit of selling an American call option.
        Uses the Cox-Ross-Rubinstein (CRR) Binomial Pricing Model adapted for early exercise.

        Args:
            S0 (float): Current stock price.
            P2 (float): Strike price of the call option.
            P3 (float): Premium received for selling the option today (user input).
            sigma (float): Annualized volatility.
            T (float): Time to expiration in years.
            r (float): Annualized risk-free interest rate.
            N (int): Number of time steps.

        Returns:
            float: The expected profit of selling the call option.
        """
        
        dt = T / N  
        u = math.exp(sigma * math.sqrt(dt))
        d = 1 / u
        p = (math.exp(r * dt) - d) / (u - d) 
        discount_factor = math.exp(-r * dt)

        # Initialize stock prices and option values at final time step (T)
        option_values = [0.0] * (N + 1)
        for i in range(N + 1):
            stock_price_at_T = S0 * (u**(N - i)) * (d**i)
            option_values[i] = max(0.0, stock_price_at_T - P2)

        # Work backwards through the tree
        for j in range(N - 1, -1, -1):
            # Calculate stock prices at the current step (needed for early exercise check)
            current_step_prices = [S0 * (u**(j - i)) * (d**i) for i in range(j + 1)]

            for i in range(j + 1):
                # Calculate expected future value (from the next step's values)
                expected_future_value = p * option_values[i] + (1 - p) * option_values[i+1]
                discounted_future_value = expected_future_value * discount_factor
                
                # --- American Option Logic ---
                # The option holder can choose to exercise *now* or wait.
                # The value of the option is the maximum of the immediate exercise value
                # and the value of waiting (discounted future value).
                immediate_exercise_value = max(0.0, current_step_prices[i] - P2)
                option_values[i] = max(immediate_exercise_value, discounted_future_value)
                # ---------------------------
                
            # Resize the option_values list for the next iteration backwards
            option_values = option_values[:j+1]

        # The value at the first node (index 0) is the fair price of the call today
        fair_call_price_V_call = option_values[0]
        
        # Calculate Expected Profit (Premium Received - Fair Value)
        expected_profit = P3 - fair_call_price_V_call
        
        return expected_profit
    
class OptionsStrategyBasic(BaseOpenStrategyV2, BaseStrategy):
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
        
        return [ ltf_repo_daily,htf_repo_daily ]
    
    def __init__(self, playground, symbol: str, logger, max_open_count: int = 3, sl_buffer=0.0, tp_buffer=0.0):
        sl_shift = 0.0
        tp_shift = 0.0
        
        super().__init__(playground, symbol, sl_shift, tp_shift, sl_buffer, tp_buffer)
        
        self.logger = logger.bind(symbol=symbol)
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
        
    def _get_feature_vector(self, new_candle: pd.DataFrame) -> Optional[Tuple[int, float, int]]:
        st_direction = new_candle.superD_50_3
        previous_supertrend_count = self.candles_ltf_idx
        for i in range(self.candles_ltf_idx-1, 0, -1):
            previous_direction = self.candles_ltf.iloc[i].superD_50_3
            if previous_direction != st_direction:
                ltf_supertrend_count = abs(previous_supertrend_count - i)
                ltf_supertrend_value = new_candle.superT_50_3
                previous_supertrend_count = i
                
                return ltf_supertrend_count, ltf_supertrend_value, st_direction
            
        return None
    
    def check_for_new_signal(self, new_candle: pd.DataFrame):
        result = self._get_feature_vector(new_candle)
        if result is None:
            return None

        ltf_supertrend_count, ltf_supertrend_value, st_direction = result
        
        signal = OpenSignalV4(
            symbol = self.symbol,
            name="SHORT_CALL_SIGNAL",
            timestamp=isoparse(new_candle.datetime),
            price=new_candle.close,
            ltf_supertrend_count=ltf_supertrend_count,
            ltf_supertrend_value=ltf_supertrend_value,
            ltf_supertrend_direction=st_direction,
            expected_volatility=0.0,
        )
        
        signal.expected_volatility = self.playground.stats.calculate_local_model_volatility(signal)
        
        return signal
            
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

    # def check_for_new_signal(self) -> OpenSignalName:
    #     latest_candle = self.candles_ltf.iloc[self.candles_ltf_idx - 1]
    #     signal = None
    #     # Only check for new signals on Mondays
    #     dt = isoparse(latest_candle['datetime'])
    #     if dt.weekday() == 0:
    #         signal = OpenSignalName.LONG_OPTION_ENTRY
        
    #     return signal

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
                    quantity_to_close=abs(position.quantity)
                )
                close_signals.append(close_signal)
                
        return close_signals
        
                               
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
            
        close_signals = self.check_for_early_close_signals(positions= self.playground.get_option_positions())

        return open_signals, close_signals

    # ------------------------------------------------------------------ #
    # BaseStrategy interface methods
    # ------------------------------------------------------------------ #

    def on_tick(self, tick_deltas) -> None:
        """Process tick deltas: generate signals and place orders internally.

        Wraps the existing tick() method and the order-placement logic
        from run_options_strategy() into a single call.
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


def calculate_stock_quantity(playground: BacktesterPlaygroundClient, stock_symbol: str, new_option_qty: float) -> float:
    options_qty = playground.get_options_quantity(stock_symbol)
    options_qty += new_option_qty
    
    if options_qty < 0:
        current_stock_position = playground.account.positions.get(stock_symbol, None)
        if current_stock_position:
            current_stock_qty = current_stock_position.quantity
        else:
            current_stock_qty = 0.0
            
        return abs(options_qty) * 100 - current_stock_qty

    return 0.0


def run(playground: BacktesterPlaygroundClient, symbol: str, logger, max_open_count: int = 3):
    strategy = OptionsStrategyBasic(playground, symbol, logger, max_open_count=max_open_count)
    
    while not strategy.is_complete():
        tick_deltas = playground.flush_new_state_buffer()
        for tick_delta in tick_deltas:
            open_signals, close_signals = strategy.tick(tick_delta)
            
            for signal in close_signals:
                playground.place_order(
                    signal.option_contract.symbol, 
                    abs(signal.quantity_to_close), 
                    OrderSide.BUY_TO_CLOSE,
                    'option',
                    attributes={ "signal_name": signal.name },
                    with_tick=True
                )
                
                logger.info(f"Close Signal: {signal.name} at {signal.timestamp} for {signal.symbol}")
            
            for signal in open_signals:
                expiration_in_days = strategy.find_next_friday(signal.timestamp)
                
                request = GetOptionsLadderRequest(
                    playground_id=playground.id,
                    stock_symbol=signal.symbol,
                    max_no_of_strikes=5,
                    min_distance_between_strikes=1.0,
                    expiration_in_days=[expiration_in_days]
                )
                
                response = playground.fetch_ladder(request)
                if response:
                    target_contract = strategy.find_target_option_contract(signal.price, response.contracts)
                        
                    if target_contract is None:
                        logger.warning(f"No suitable option contract found for {signal.symbol} at price {signal.price}")
                        continue
                    
                    stock_qty = calculate_stock_quantity(playground, signal.symbol, -1)
                    if stock_qty > 0:
                        playground.place_order(
                            signal.symbol,
                            stock_qty,
                            OrderSide.BUY,
                            'equity',
                            signal.price
                        )
                    
                    playground.place_order(
                        target_contract.symbol, 
                        1, 
                        OrderSide.SELL_TO_OPEN,
                        'option',
                        attributes={ "signal_name": signal.name }
                    )
                    
                    logger.info(f"Open Signal: {signal.name} at {signal.timestamp} for {signal.symbol}")
                
        playground.tick(playground.ltf_seconds)
        

    logger.info("Done")
    
def find_option_contract_by_extrinsic_threshold(playground: BacktesterPlaygroundClient, symbol: str, underlying_price: float, strike: float, y_percent_threshold: float) -> OptionLadderContract:
    """
    Given:
        symbol (str) – the option underlying (e.g., "AAPL")
        underlying_price (float) – current price of the underlying asset
        strike (float) – strike price of interest
        y_percent_threshold (float) – extrinsic value must be <= this percent (0.0–1.0)

    Returns:
        OptionLadderContract with expiration_date (date) where extrinsic_value / option_price <= y_percent_threshold
        or None if not found.
    """
    min_days_in_future = 90
    max_days_in_future = 360
    best_contract = (1.0, None, None, None, None) # (extrinsic_ratio, contract, intrinsic_value, extrinsic_value, extrinsic_ratio)
    max_diff = 1.0
    
    for days_in_future in range(min_days_in_future, max_days_in_future + 1, 30):
        request = GetOptionsLadderRequest(
            playground_id=playground.id,
            stock_symbol=symbol,
            max_no_of_strikes=20,
            min_distance_between_strikes=10.0,
            expiration_in_days=[days_in_future],
            max_tick_age_in_minutes=1440,
            base_strike_price=strike
        )
        
        response = playground.fetch_ladder(request)
        if response is None or len(response.contracts) == 0:
            continue
        
        for contract in response.contracts:
            if contract.strike > strike:
                continue
            
            if contract.type != 'call':
                continue
            
            option_price = (contract.bid + contract.ask) / 2
            if option_price <= 0:
                continue
            
            intrinsic_value = max(0.0, underlying_price - contract.strike)
            extrinsic_value = option_price - intrinsic_value
            
            extrinsic_ratio = extrinsic_value / option_price
            diff = y_percent_threshold - extrinsic_ratio
            if diff >= 0 and diff < max_diff:
                max_diff = diff
                best_contract = (extrinsic_ratio, contract, intrinsic_value, extrinsic_value, extrinsic_ratio)

    if best_contract[1] is not None:
        return best_contract[1], best_contract[2], best_contract[3], best_contract[4], 'best_found'
            
    return None, None, None, None, None
   
   
def long_call_selection_option(playground: BacktesterPlaygroundClient, symbol: str):
    y_percent_threshold = 0.15
    
    underlying_bar = playground.fetch_closest_bar(symbol, playground.ltf_seconds, playground.timestamp)
    underlying_price = underlying_bar.close
    strike = underlying_bar.superT_50_3
    
    print(f"Searching for option contract for {symbol} with strike near {strike:.2f} and extrinsic ratio <= {y_percent_threshold:.2f}, with underlying price {underlying_price:.2f}")
    
    contract, intrinsic_value, extrinsic_value, extrinsic_ratio, status = find_option_contract_by_extrinsic_threshold(playground, symbol, underlying_bar.close, strike, y_percent_threshold)
    
    if contract is not None:
        if status == 'threshold_met':
            print(f"Found contract: {contract}, Intrinsic Value: {intrinsic_value}, Extrinsic Value: {extrinsic_value}, Extrinsic Ratio: {extrinsic_ratio}")
        elif status == 'best_found':
            print(f"No contract met the threshold. Best found contract: {contract}, Intrinsic Value: {intrinsic_value}, Extrinsic Value: {extrinsic_value}, Extrinsic Ratio: {extrinsic_ratio}")
        else:
            print(f"Unknown status.")
    else:
        print(f"No contracts found.")


@dataclass
class Signal:
    open_signal_name: str
    open_timestamp: datetime
    open_price: float
    close_signal_name: str
    close_timestamp: datetime
    close_price: float
    ltf_supertrend_count: int
    ltf_supertrend_value: float
    ltf_supertrend_direction: int
    
    def log_return(self) -> float:
        return math.log(self.close_price / self.open_price)
    
    def duration_minutes(self) -> float:
        delta = self.duration()
        return delta.total_seconds() / 60
    
    def duration(self):
        return self.close_timestamp - self.open_timestamp
    
class SignalStats:
    def __init__(self):
        self.signals: List[Signal] = []
        
    def __len__(self):
        return len(self.signals)
        
    def add_signal(self, signal: Signal):
        self.signals.append(signal)
    
     # Gaussian kernel
    def _kernel(self, u: float) -> float:
        return math.exp(-0.5 * u * u) / math.sqrt(2 * math.pi)
    
    # Local model EV estimator
    def _local_model_ev(self, x0: float, xs: np.ndarray, ys: np.ndarray, bandwidth: float) -> float:
        u = (xs - x0) / bandwidth
        weights = np.array([self._kernel(ui) for ui in u])
        weights /= weights.sum() if weights.sum() != 0 else 1.0
        return float(np.sum(weights * ys))
    
    def _multivariate_local_model_volatility(self, x0: np.ndarray, X: np.ndarray, y: np.ndarray, bandwidth: float) -> float:
        """
        Multivariate kernel regression for volatility estimation
        """
        if len(X) == 0:
            return 0.0
        
        # Calculate Mahalanobis-like distance or use different bandwidths per dimension
        distances = np.linalg.norm((X - x0) / bandwidth, axis=1)
        weights = np.array([self._kernel(d) for d in distances])
        
        if weights.sum() == 0:
            return np.std(y)  # Fallback to global stddev
        
        weights /= weights.sum()
        mean = float(np.sum(weights * y))
        weighted_variance = float(np.sum(weights * (y - mean) ** 2))
        
        # Optional: Apply Bessel's correction for sample size if desired, but 
        # for large financial datasets, simple weighted variance works well.
        # To be precise, adjust for effective sample size:
        effective_n = 1 / np.sum(weights**2)
        if effective_n > 1:
            adjusted_variance = weighted_variance * (effective_n / (effective_n - 1))
        else:
            adjusted_variance = weighted_variance

        # 3. Volatility is the square root of the variance
        volatility = np.sqrt(adjusted_variance)
        
        return float(volatility)
    
    def _multivariate_local_model_ev(self, x0: np.ndarray, X: np.ndarray, y: np.ndarray, bandwidth: float) -> float:
        """
        Multivariate kernel regression
        """
        if len(X) == 0:
            return 0.0
        
        # Calculate Mahalanobis-like distance or use different bandwidths per dimension
        distances = np.linalg.norm((X - x0) / bandwidth, axis=1)
        weights = np.array([self._kernel(d) for d in distances])
        
        if weights.sum() == 0:
            return np.mean(y)  # Fallback to global mean
        
        weights /= weights.sum()
        return float(np.sum(weights * y))
    
    def _adaptive_bandwidth(self, x0: np.ndarray, X: np.ndarray) -> np.ndarray:
        """
        Use different bandwidths for different dimensions based on data density
        """
        bandwidths = []
        for i in range(len(x0)):
            # Use standard deviation of dimension i as bandwidth
            std_i = np.std(X[:, i])
            bandwidths.append(max(std_i, 0.1))  # Minimum bandwidth
        return np.array(bandwidths)
    
    def generate_model(self):
        if not self.signals:
            return None
        
        # Create multi-dimensional features
        features = []
        log_returns = []
        
        for s in self.signals:
            feature_vector = np.array([
                s.ltf_supertrend_count,
                s.ltf_supertrend_value,
                s.ltf_supertrend_direction
            ])
            
            features.append(feature_vector)
            log_returns.append(s.log_return())
            
        self.features = np.array(features)
        self.log_returns = np.array(log_returns)
        
    def calculate_local_model_volatility(self, signal: OpenSignalV4, bandwidth: float = 30.0) -> float:
        """
        bandwidth in minutes for local duration-based modeling
        """
        if not self.signals:
            raise Exception("No signals to compute volatility.")
        
        x0 = np.array([
            signal.ltf_supertrend_count,
            signal.ltf_supertrend_value,
            signal.ltf_supertrend_direction
        ])
        
        expected_volatility = self._multivariate_local_model_volatility(x0, self.features, self.log_returns, bandwidth)
        return expected_volatility
    
    def calculate_local_model_expected_log_returns(self, signal: OpenSignalV4, bandwidth: float = 30.0) -> float:
        """
        bandwidth in minutes for local duration-based modeling
        """
        if not self.signals:
            raise Exception("No signals to compute EV.")
        
        x0 = np.array([
            signal.ltf_supertrend_count,
            signal.ltf_supertrend_value,
            signal.ltf_supertrend_direction
        ])
        
        expected_log_returns = self._multivariate_local_model_ev(x0, self.features, self.log_returns, bandwidth)
        return expected_log_returns
    
    def compute_stats(self, bandwidth: float = 30.0):
        """
        bandwidth in minutes for local duration-based modeling
        """
        ny_tz = ZoneInfo("America/New_York")
        
        results = []
        for i, s in enumerate(self.signals):
            x0 = self.features[i]
            local_ev = self._multivariate_local_model_ev(x0, self.features, self.log_returns, bandwidth)
            results.append({
                "open_time": s.open_timestamp.astimezone(ny_tz).strftime("%Y-%m-%d %H:%M:%S %Z"),
                "open_price": s.open_price,
                "close_time": s.close_timestamp.astimezone(ny_tz).strftime("%Y-%m-%d %H:%M:%S %Z"),
                "close_price": s.close_price,
                "duration_min": s.duration_minutes(),
                "log_return": s.log_return(),
                "local_ev": local_ev
            })

        return results
    

def generate_csv(playground: BacktesterPlaygroundClient, symbol: str):
    candles = playground.fetch_candles_v3(symbol, playground.ltf_seconds, playground.timestamp - timedelta(days=365), playground.timestamp)
    data = []
    for candle in candles:
        data.append({
            'datetime': candle.datetime,
            'open': candle.open,
            'high': candle.high,
            'low': candle.low,
            'close': candle.close,
            'volume': candle.volume,
            'up trend': candle.superL_50_3,
            'down trend': candle.superD_50_3,
            'atr': candle.atr_14,
            'cdl_doji': candle.cdl_doji_10_0_1,
            'cdl_hammer': candle.cdl_hammer
        })
        
    df = pd.DataFrame(data)
    csv_filename = f"{symbol}_candles.csv"
    df.to_csv(csv_filename, index=False)
    print(f"CSV file generated: {csv_filename}")    

def is_after_market_open(candle_dt: datetime) -> bool:
    # Parse and convert to NY timezone
    ny_tz = ZoneInfo("America/New_York")
    candle_dt = candle_dt.astimezone(ny_tz)
    
    # Check if time is after 9:30 AM
    market_open = time(9, 30)  # 9:30 AM
    return candle_dt.time() >= market_open


# atr_14, cdl_doji_10_0_1, cdl_hammer
def generate_signal_stats(playground: BacktesterPlaygroundClient, symbol: str) -> SignalStats:
    # htf_candles = playground.fetch_candles_v3(symbol, playground.htf_seconds, playground.timestamp - timedelta(days=365), playground.timestamp, calculate_is_extended_hours=True)
    ltf_candles = playground.fetch_candles_v3(symbol, playground.ltf_seconds, playground.timestamp - timedelta(days=365), playground.timestamp, calculate_is_extended_hours=True)
    stats = SignalStats()
    initial_st_direction = ltf_candles[0].superD_50_3
    previous_st_direction = None
    bars_since_st_change = -1
    for i in range(1, len(ltf_candles)):
        open_candle = ltf_candles[i]
        if open_candle.is_extended_hours:
            continue
        
        st_direction = ltf_candles[i].superD_50_3
        
        if previous_st_direction is None:
            if st_direction == initial_st_direction:
                continue
            
        if st_direction != previous_st_direction:
            bars_since_st_change = 0
        else:
            bars_since_st_change += 1
                    
        previous_st_direction = st_direction
            
        # Find close candle at end of the week (Friday)
        close_candle = None
        for j in range(i + 1, len(ltf_candles)):
            candle = ltf_candles[j]
            candle_dt = isoparse(candle.datetime)
            
            candle_day_of_week = candle_dt.weekday()
            if candle_day_of_week == 4 and is_after_market_open(candle_dt):
                if candle.is_extended_hours:
                    close_candle = ltf_candles[j - 1]
                else:
                    close_candle = candle
                break
            
        if close_candle is None:
            continue
        
        stats.add_signal(Signal(
            open_signal_name="TEST_OPEN",
            open_timestamp=isoparse(open_candle.datetime),
            open_price=open_candle.open,
            close_signal_name="TEST_CLOSE",
            close_timestamp=isoparse(close_candle.datetime),
            close_price=close_candle.close,
            ltf_supertrend_count=bars_since_st_change,
            ltf_supertrend_value=open_candle.superT_50_3,
            ltf_supertrend_direction=open_candle.superD_50_3
        ))

    print(f'Generating CDF for {symbol} over last {len(stats)} candles')
    
    # results = stats.compute_stats(bandwidth=60.0)
    # if results is None:
    #     raise Exception("No signals to compute stats.")
    
    # df = pd.DataFrame(results)
    # cdf_filename = f"{symbol}_signal_stats.csv"
    # df.to_csv(cdf_filename, index=False)
    # print(f"CDF file generated: {cdf_filename}")
    
    playground.stats = stats
    
def run_options_strategy(playground: BacktesterPlaygroundClient, symbol: str, logger, twirp_host: str):
    generate_signal_stats(playground, symbol)
    
    playground.stats.generate_model()
    
    max_open_count = 3
    strategy = OptionsStrategyBasic(playground, symbol, logger, max_open_count=max_open_count)
        
    while not strategy.is_complete():
        tick_deltas = playground.flush_new_state_buffer()
        for tick_delta in tick_deltas:
            open_signals, close_signals = strategy.tick(tick_delta)
            for signal in close_signals:
                playground.place_order(
                    signal.option_contract.symbol, 
                    abs(signal.quantity_to_close), 
                    OrderSide.BUY_TO_CLOSE,
                    'option',
                    # attributes={ "signal_name": signal.name },
                    with_tick=True
                )
                
                logger.info(f"Close Signal: {signal.name} at {signal.timestamp} for {signal.option_contract.symbol}")
            
            for signal in open_signals:
                if isinstance(signal, OpenSignalV4):
                    expiration_in_days = strategy.find_next_friday(signal.timestamp)
                elif isinstance(signal, RollSignalV1):
                    expiration_in_days = strategy.find_next_friday(signal.option_contract.expiration_date)
                else:
                    raise Exception("Unknown signal type.")
                
                
                if isinstance(signal, RollSignalV1):
                    request = GetOptionsLadderRequest(
                        playground_id=playground.id,
                        stock_symbol=signal.symbol,
                        max_no_of_strikes=5,
                        min_distance_between_strikes=1.0,
                        expiration_in_days=[expiration_in_days, expiration_in_days + 7, expiration_in_days + 14],
                        max_tick_age_in_minutes=1440
                    )
                else:
                    request = GetOptionsLadderRequest(
                        playground_id=playground.id,
                        stock_symbol=signal.symbol,
                        max_no_of_strikes=5,
                        min_distance_between_strikes=1.0,
                        expiration_in_days=[expiration_in_days],
                        max_tick_age_in_minutes=1440
                    )
                    
                response = playground.fetch_ladder(request)
                if response:
                    # target_contract = strategy.find_target_option_contract(
                    #     current_price=signal.price,
                    #     contracts=response.contracts
                    # )
                    target_contract = None
                    highest_expected_profit = -1
                    for c in response.contracts:
                        if c.type != 'call':
                            continue
                        
                        S0 = signal.price
                        strike_price = c.strike
                        premium_received = (c.bid + c.ask) / 2  # Replace with actual premium received
                        T = expiration_in_days / 365.0
                        sigma = playground.stats.calculate_local_model_volatility(signal)
                        profit_american = calculate_expected_profit_binomial_american(
                            S0=S0,
                            P2=strike_price,
                            P3=premium_received,  # Replace with actual premium received
                            sigma=sigma,  # Replace with actual volatility
                            T=T,  # Replace with actual time to expiration
                            r=0.04,  # Replace with actual risk-free rate
                            N=100  # Replace with actual number of time steps
                        )

                        if isinstance(signal, RollSignalV1):
                            position = playground.account.positions.get(signal.option_contract.symbol, None)
                            if position is None:
                                logger.warn(f"No position found for {signal.option_contract.symbol} while processing roll signal.")
                                continue
                            current_price = position.current_price
                            if not signal.name.startswith('ROLL_CALL_LOSS_RATIO'):
                                if premium_received - current_price < 0:
                                    continue  # Do not roll at a loss
                            
                        if profit_american > highest_expected_profit:
                            highest_expected_profit = profit_american
                            target_contract = c
                                                
                    if target_contract is None:
                        logger.warning(f"No suitable option contract found for {signal.symbol} at price {signal.price}")
                        continue
                    
                    attributes = { "ev": str(highest_expected_profit), "stock_price": str(signal.price), "signal_name": signal.name }
                    
                    if signal.__class__ == RollSignalV1:
                        playground.place_order(
                            signal.option_contract.symbol,
                            signal.quantity_to_roll,
                            OrderSide.BUY_TO_CLOSE,
                            'option'
                        )
                        
                        attributes["roll_from"] = signal.option_contract.symbol
                    else:
                        stock_qty = calculate_stock_quantity(playground, signal.symbol, -1)
                        if stock_qty > 0:
                            playground.place_order(
                                signal.symbol,
                                stock_qty,
                                OrderSide.BUY,
                                'equity',
                                signal.price
                            )
                    
                    if isinstance(signal, OpenSignalV4):
                        p = playground.account.get_position(target_contract.symbol)
                        if p is not None and p.quantity != 0:
                            logger.warning(f"Skip open an option position for {target_contract.symbol} but position already exists with quantity {p.quantity}. Skipping order.")
                            continue
                    
                    # In order to calculate the option EV, we 
                    playground.place_order(
                        target_contract.symbol, 
                        1, 
                        OrderSide.SELL_TO_OPEN,
                        'option',
                        attributes=attributes
                    )
                    
                    logger.info(f"Open Signal: {signal.name} at {signal.timestamp} for {signal.symbol}")
                
        playground.tick(playground.ltf_seconds)
        logger.info(f"Ticked playground to {playground.timestamp.isoformat()}")
        

    logger.info(f"Done - playground id: {playground.id}")

def generate_random_market_times(count, target_date=None):
    """
    Generate random datetime objects between 9:30 AM and 4:00 PM for a given date.
    
    Args:
        count: number of random times to generate
        target_date: date object to use (defaults to today)
    
    Returns:
        List of datetime objects
    """
    if target_date is None:
        target_date = date.today()
    
    times = []
    # Market opens at 9:30 AM (570 minutes from midnight)
    # Market closes at 4:00 PM (960 minutes from midnight)
    start_minutes = 9 * 60 + 30  # 570 minutes
    end_minutes = 16 * 60        # 960 minutes
    
    for _ in range(count):
        total_minutes = randint(start_minutes, end_minutes)
        hour = total_minutes // 60
        minute = total_minutes % 60
        
        # Create datetime object
        dt = datetime.combine(target_date, datetime.min.time().replace(hour=hour, minute=minute))
        times.append(dt.astimezone(ZoneInfo("America/New_York")))
    
    return sorted(times)

def generate_random_signals(playground: BacktesterPlaygroundClient, symbol: str, logger):
    trades_remaining_per_day = 3
    trade_open_time_in_minutes = 60
    
    day_of_week = playground.timestamp.weekday()
    random_market_times = generate_random_market_times(trades_remaining_per_day, target_date=playground.timestamp.date())
    current_trade_index = 0
    
    while not playground.is_backtest_complete():
        # Check for open orders to close
        open_orders = playground.fetch_open_orders(symbol)
        for order in open_orders:
            clean_str = order.create_date.rsplit(' ', 1)[0]
            order_create_dt = parser.parse(clean_str)
            order_age = (playground.timestamp - order_create_dt).total_seconds() / 60
            if order_age >= trade_open_time_in_minutes:
                # logger.info(f"Closing open order for {order.symbol} placed at {clean_str}")
                playground.place_order(
                    order.symbol,
                    order.quantity,
                    OrderSide.SELL if order.side == OrderSide.BUY.value else OrderSide.BUY_TO_COVER,
                    getattr(order, 'class'),
                    close_order_id=order.id,
                    with_tick=True
                )
        
        # Check for open orders
        if playground.timestamp.weekday() != day_of_week:
            day_of_week = playground.timestamp.weekday()
            random_market_times = generate_random_market_times(trades_remaining_per_day, target_date=playground.timestamp.date())
            current_trade_index = 0
        
        if current_trade_index > len(random_market_times) - 1:
            playground.tick(playground.ltf_seconds)
            continue
        
        next_trade_time = random_market_times[current_trade_index]
        if playground.timestamp >= next_trade_time:
            # logger.info(f"Placing random trade at {playground.timestamp.isoformat()}")
            playground.place_order(
                symbol,
                100,
                OrderSide.BUY,
                'equity',
                playground.get_current_candle(symbol, playground.ltf_seconds).close
            )
            current_trade_index += 1
        
        playground.tick(playground.ltf_seconds)
        
def parse_with_tz_info(dt_str):
    # Remove the timezone abbreviation for initial parsing
    clean_str = dt_str.rsplit(' ', 1)[0]  # '2023-12-19 10:30:00 -0500'
    
    # Parse with dateutil
    dt = parser.parse(clean_str)
    
    # Extract timezone abbreviation
    tz_abbr = dt_str.split()[-1]  # 'EST'
    
    # Map to proper timezone
    tz_map = {
        'EST': tz.gettz('America/New_York'),
        'EDT': tz.gettz('America/New_York'),
        'CST': tz.gettz('America/Chicago'),
        'CDT': tz.gettz('America/Chicago'),
        'MST': tz.gettz('America/Denver'),
        'MDT': tz.gettz('America/Denver'),
        'PST': tz.gettz('America/Los_Angeles'),
        'PDT': tz.gettz('America/Los_Angeles'),
    }
    
    if tz_abbr in tz_map:
        # Convert to the proper timezone
        dt = dt.astimezone(tz_map[tz_abbr])
    
    return dt

def calc_highest_log_profit(playground: BacktesterPlaygroundClient, order) -> Tuple[float, datetime]:
    highest_log_profit = -float('inf')
    highest_log_profit_timestamp = None
    
    create_date_dt = parse_with_tz_info(order.create_date)
    candles = playground.fetch_candles_v3(order.symbol, playground.ltf_seconds, create_date_dt, playground.timestamp)
    for candle in candles:
        current_price = candle.close
        open_price = get_vwap(order)
        
        if order.side == OrderSide.BUY.value:
            log_profit = math.log(current_price / open_price)
            if log_profit > highest_log_profit:
                highest_log_profit = log_profit
                highest_log_profit_timestamp = isoparse(candle.datetime)
        elif order.side == OrderSide.SELL_SHORT.value:
            log_profit = math.log(open_price / current_price)
            if log_profit > highest_log_profit:
                highest_log_profit = log_profit
                highest_log_profit_timestamp = isoparse(candle.datetime)
        else:
            raise Exception(f"Unknown order side: {order.side}")
    
    return highest_log_profit, highest_log_profit_timestamp

def get_vwap(order) -> float:
    vwap = 0.0
    for trade in order.trades:
        vwap += trade.price * trade.quantity
    vwap /= sum(trade.quantity for trade in order.trades)
    
    return vwap
        
def generate_max_profit_random(playground: BacktesterPlaygroundClient, symbol: str, logger):
    trades_remaining_per_day = 3
    trade_open_time_in_minutes = 60
    
    day_of_week = playground.timestamp.weekday()
    random_market_times = generate_random_market_times(trades_remaining_per_day, target_date=playground.timestamp.date())
    current_trade_index = 0
    
    while not playground.is_backtest_complete():
        # Check for open orders to close
        open_orders = playground.fetch_open_orders(symbol)
        for order in open_orders:
            current_price = playground.get_current_candle(symbol, playground.ltf_seconds).close
            open_price = get_vwap(order)
            log_profit = math.log(current_price / open_price)
            if log_profit < -0.4:
                highest_log_profit, highest_log_profit_timestamp = calc_highest_log_profit(playground, order)
                highest_log_profit_timestamp_str = highest_log_profit_timestamp.isoformat()
                attributes = { 'highest_log_profit': str(highest_log_profit), 'highest_log_profit_timestamp': highest_log_profit_timestamp_str }
                
                playground.place_order(
                    order.symbol,
                    order.quantity,
                    OrderSide.SELL if order.side == OrderSide.BUY.value else OrderSide.BUY_TO_COVER,
                    getattr(order, 'class'),
                    close_order_id=order.id,
                    with_tick=True,
                    attributes=attributes
                )
        
        # Check for open orders
        if playground.timestamp.weekday() != day_of_week:
            day_of_week = playground.timestamp.weekday()
            random_market_times = generate_random_market_times(trades_remaining_per_day, target_date=playground.timestamp.date())
            current_trade_index = 0
        
        if current_trade_index > len(random_market_times) - 1:
            playground.tick(playground.ltf_seconds)
            continue
        
        next_trade_time = random_market_times[current_trade_index]
        if playground.timestamp >= next_trade_time:
            # logger.info(f"Placing random trade at {playground.timestamp.isoformat()}")
            playground.place_order(
                symbol,
                1,
                OrderSide.SELL_SHORT,
                'equity',
                playground.get_current_candle(symbol, playground.ltf_seconds).close
            )
            current_trade_index += 1
        
        playground.tick(playground.ltf_seconds)
        
    return 'Max Profit Random - SHORT'
        
        
def generate_max_profit_v2(playground: BacktesterPlaygroundClient, symbol: str, logger):
    trades_remaining_per_day = 3
    trade_open_time_in_minutes = 60
    
    day_of_week = playground.timestamp.weekday()
    random_market_times = generate_random_market_times(trades_remaining_per_day, target_date=playground.timestamp.date())
    current_trade_index = 0
    
    while not playground.is_backtest_complete():
        # Check for open orders to close
        open_orders = playground.fetch_open_orders(symbol)
        for order in open_orders:
            current_price = playground.get_current_candle(symbol, playground.ltf_seconds).close
            open_price = get_vwap(order)
            log_profit = math.log(current_price / open_price)
            if log_profit < -0.4:
                highest_log_profit, highest_log_profit_timestamp = calc_highest_log_profit(playground, order)
                highest_log_profit_timestamp_str = highest_log_profit_timestamp.isoformat()
                attributes = { 'highest_log_profit': str(highest_log_profit), 'highest_log_profit_timestamp': highest_log_profit_timestamp_str }
                
                playground.place_order(
                    order.symbol,
                    order.quantity,
                    OrderSide.SELL if order.side == OrderSide.BUY.value else OrderSide.BUY_TO_COVER,
                    getattr(order, 'class'),
                    close_order_id=order.id,
                    with_tick=True,
                    attributes=attributes
                )
        
        # Check for open orders
        if playground.timestamp.weekday() != day_of_week:
            day_of_week = playground.timestamp.weekday()
            random_market_times = generate_random_market_times(trades_remaining_per_day, target_date=playground.timestamp.date())
            current_trade_index = 0
        
        if current_trade_index > len(random_market_times) - 1:
            playground.tick(playground.ltf_seconds)
            continue
        
        try:
            current_htf_candle = playground.get_current_candle(symbol, playground.htf_seconds)
        except Exception as e:
            continue
        
        if current_htf_candle.superD_50_3 != -1:
            playground.tick(playground.ltf_seconds)
            continue
        
        try:
            current_ltf_candle = playground.get_current_candle(symbol, playground.ltf_seconds)
        except Exception as e:
            playground.tick(playground.ltf_seconds)
            continue
        
        if current_ltf_candle.superD_50_3 != -1:
            playground.tick(playground.ltf_seconds)
            continue
        
        next_trade_time = random_market_times[current_trade_index]
        if playground.timestamp >= next_trade_time:
            # logger.info(f"Placing random trade at {playground.timestamp.isoformat()}")
            playground.place_order(
                symbol,
                1,
                OrderSide.SELL_SHORT,
                'equity',
                playground.get_current_candle(symbol, playground.ltf_seconds).close
            )
            current_trade_index += 1
        
        playground.tick(playground.ltf_seconds)
        
    return 'Max Profit v2 - SHORT (-1, -1)'
        
def generate_max_profit_v3(playground: BacktesterPlaygroundClient, symbol: str, logger):
    trades_remaining_per_day = 3
    trade_open_time_in_minutes = 60
    
    day_of_week = playground.timestamp.weekday()
    random_market_times = generate_random_market_times(trades_remaining_per_day, target_date=playground.timestamp.date())
    current_trade_index = 0
    
    while not playground.is_backtest_complete():
        # Check for open orders to close
        open_orders = playground.fetch_open_orders(symbol)
        for order in open_orders:
            current_price = playground.get_current_candle(symbol, playground.ltf_seconds).close
            open_price = get_vwap(order)
            log_profit = math.log(current_price / open_price)
            if log_profit < -0.1:
                highest_log_profit, highest_log_profit_timestamp = calc_highest_log_profit(playground, order)
                highest_log_profit_timestamp_str = highest_log_profit_timestamp.isoformat()
                attributes = { 'highest_log_profit': str(highest_log_profit), 'highest_log_profit_timestamp': highest_log_profit_timestamp_str }
                
                playground.place_order(
                    order.symbol,
                    order.quantity,
                    OrderSide.SELL if order.side == OrderSide.BUY.value else OrderSide.BUY_TO_COVER,
                    getattr(order, 'class'),
                    close_order_id=order.id,
                    with_tick=True,
                    attributes=attributes
                )
        
        # Check for open orders
        if playground.timestamp.weekday() != day_of_week:
            day_of_week = playground.timestamp.weekday()
            random_market_times = generate_random_market_times(trades_remaining_per_day, target_date=playground.timestamp.date())
            current_trade_index = 0
        
        if current_trade_index > len(random_market_times) - 1:
            playground.tick(playground.ltf_seconds)
            continue
        
        try:
            current_htf_candle = playground.get_current_candle(symbol, playground.htf_seconds)
        except Exception as e:
            continue
        
        if current_htf_candle.superD_50_3 != -1:
            playground.tick(playground.ltf_seconds)
            continue
        
        try:
            current_ltf_candle = playground.get_current_candle(symbol, playground.ltf_seconds)
        except Exception as e:
            playground.tick(playground.ltf_seconds)
            continue
        
        if current_ltf_candle.superD_50_3 != 1:
            playground.tick(playground.ltf_seconds)
            continue
        
        next_trade_time = random_market_times[current_trade_index]
        if playground.timestamp >= next_trade_time:
            # logger.info(f"Placing random trade at {playground.timestamp.isoformat()}")
            playground.place_order(
                symbol,
                1,
                OrderSide.SELL_SHORT,
                'equity',
                playground.get_current_candle(symbol, playground.ltf_seconds).close
            )
            current_trade_index += 1
        
        playground.tick(playground.ltf_seconds)
        
    return 'Max Profit v3 - SHORT (-1, 1)'

def generate_multiple_timeframe_signals(playground: BacktesterPlaygroundClient, symbol: str, logger) -> str:
    trades_remaining_per_day = 3
    trade_open_time_in_minutes = 60
    
    day_of_week = playground.timestamp.weekday()
    random_market_times = generate_random_market_times(trades_remaining_per_day, target_date=playground.timestamp.date())
    current_trade_index = 0
    
    while not playground.is_backtest_complete():
        # Check for open orders to close
        open_orders = playground.fetch_open_orders(symbol)
        for order in open_orders:
            clean_str = order.create_date.rsplit(' ', 1)[0]
            order_create_dt = parser.parse(clean_str)
            order_age = (playground.timestamp - order_create_dt).total_seconds() / 60
            if order_age >= trade_open_time_in_minutes:
                playground.place_order(
                    order.symbol,
                    order.quantity,
                    OrderSide.SELL if order.side == OrderSide.BUY.value else OrderSide.BUY_TO_COVER,
                    getattr(order, 'class'),
                    close_order_id=order.id,
                    with_tick=True
                )
        
        # Check for open orders
        if playground.timestamp.weekday() != day_of_week:
            day_of_week = playground.timestamp.weekday()
            random_market_times = generate_random_market_times(trades_remaining_per_day, target_date=playground.timestamp.date())
            current_trade_index = 0
        
        if current_trade_index > len(random_market_times) - 1:
            playground.tick(playground.ltf_seconds)
            continue
        
        try:
            current_htf_candle = playground.get_current_candle(symbol, playground.htf_seconds)
        except Exception as e:
            continue
        
        if current_htf_candle.superD_50_3 != 1:
            playground.tick(playground.ltf_seconds)
            continue
        
        try:
            current_ltf_candle = playground.get_current_candle(symbol, playground.ltf_seconds)
        except Exception as e:
            playground.tick(playground.ltf_seconds)
            continue
        
        if current_ltf_candle.superD_50_3 != -1:
            playground.tick(playground.ltf_seconds)
            continue
        
        next_trade_time = random_market_times[current_trade_index]
        if playground.timestamp >= next_trade_time:
            # logger.info(f"Placing random trade at {playground.timestamp.isoformat()}")
            playground.place_order(
                symbol,
                100,
                OrderSide.BUY,
                'equity',
                playground.get_current_candle(symbol, playground.ltf_seconds).close
            )
            current_trade_index += 1
        
        playground.tick(playground.ltf_seconds)
        
        return 'SHORT (1, -1)'

def calc_max_log_profit(playground: BacktesterPlaygroundClient, logger):
    max_log_profits = []
    all_orders = playground.fetch_orders()
    
    for order in all_orders:
        if order.side in [OrderSide.SELL.value, OrderSide.BUY_TO_COVER.value]:
            highest_log_profit = order.attributes.get('highest_log_profit', None)
            if highest_log_profit is None:
                raise Exception(f"Order {order.id} missing highest_log_profit attribute.")
            
            max_log_profits.append(float(highest_log_profit))
            
    logger.info(f"Computed max log profits for {len(max_log_profits)} closed orders.")
    return max_log_profits

def calc_log_returns(playground: BacktesterPlaygroundClient, logger):
    log_returns = []
    all_orders = playground.fetch_orders()
    
    for order in all_orders:
        if order.side in [OrderSide.SELL.value, OrderSide.BUY_TO_COVER.value]:
            assert len(order.trades) == 1, "Expected exactly one trade per order"
            close_price = order.trades[0].price
            
            assert len(order.closes) == 1, "Expected exactly one close per order"
            assert len(order.closes[0].trades) == 1, "Expected exactly one trade per close"
            open_price = order.closes[0].trades[0].price
            
            log_return = math.log(close_price / open_price)
            log_returns.append(log_return)
            
    logger.info(f"Computed log returns for {len(log_returns)} closed orders.")
    return log_returns
    
    
if __name__ == "__main__":
    balance = 1000000
    symbol = 'AAPL'
    start_date = '2023-12-19'
    end_date = '2025-12-12'
    repository_source = RepositorySource.POLYGON
    csv_path = None
    twirp_host = 'http://127.0.0.1:5051'
    
    repos = OptionsStrategyBasic.get_repositories(symbol, datetime.fromisoformat(start_date), datetime.fromisoformat(end_date))
    
    req = CreatePolygonPlaygroundRequest(
        balance=balance,
        start_date=start_date,
        stop_date=end_date,
        repositories=repos,
        environment=PlaygroundEnvironment.SIMULATOR.value
    )
    
    live_account_type = None
    
    profitRunnersPlayground = BacktesterPlaygroundClient(req, live_account_type, repository_source, logger, twirp_host=twirp_host)
    
    # run_options_strategy(playground, logger, twirp_host)
    
    # generate_multiple_timeframe_signals(multiTimeframePlaygroundA, symbol, logger)
    
    data1_label = generate_max_profit_v2(profitRunnersPlayground, symbol, logger)
    
    data1 = np.array(calc_max_log_profit(profitRunnersPlayground, logger))
    
    multiTimeframePlaygroundA = BacktesterPlaygroundClient(req, live_account_type, repository_source, logger, twirp_host=twirp_host)
    
    # generate_random_signals(randomPlayground, symbol, logger)
    # data2 = np.array(calc_log_returns(randomPlayground, logger))
    
    data2_label = generate_max_profit_random(multiTimeframePlaygroundA, symbol, logger)
    
    data2 = np.array(calc_max_log_profit(multiTimeframePlaygroundA, logger))
    
    data1 = pd.Series(data1)
    data1 = data1.replace([np.inf, -np.inf], np.nan).dropna()
    data2 = pd.Series(data2)
    data2 = data2.replace([np.inf, -np.inf], np.nan).dropna()
    
    # Calculate basic statistics
    mean1, mean2 = data1.mean(), data2.mean()
    median1, median2 = data1.median(), data2.median()
    std1, std2 = data1.std(), data2.std()
    logger.info(f"{data1_label} - Mean: {mean1}, Median: {median1}, Std Dev: {std1}")
    logger.info(f"{data2_label} - Mean: {mean2}, Median: {median2}, Std Dev: {std2}")
    
    total_profits_1 = data1.sum()
    total_profits_2 = data2.sum()
    logger.info(f"{data1_label} - Total Log Profit: {total_profits_1}")
    logger.info(f"{data2_label} - Total Log Profit: {total_profits_2}")
    
    # Perform Kolmogorov-Smirnov test
    result = ks_2samp(data1, data2)
    stat = result.statistic
    p_value = result.pvalue
    logger.info(f"KS Statistic: {stat}, P-value: {p_value}")
    if p_value < 0.05:
        logger.info("The distributions are significantly different (reject H0).")
    else:
        logger.info("The distributions are not significantly different (fail to reject H0).")
    
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(14, 6))
    
    # --- Plot A: Histogram with KDE ---
    # Use stat='density' and common_norm=False to compare different sample sizes fairly
    sns.histplot(data1, color="skyblue", label=data1_label, kde=True, stat="density", ax=ax1, alpha=0.5)
    sns.histplot(data2, color="orange", label=data2_label, kde=True, stat="density", ax=ax1, alpha=0.5)
    ax1.set_title("Distribution Comparison (KDE Overlay)")
    ax1.set_xlabel("Log Returns")
    ax1.legend()
    
    # --- Plot B: Two-Sample Q-Q Plot ---
    # If points fall on the 45-degree line, the distributions are identical
    sm.qqplot_2samples(data1, data2, line='45', ax=ax2)
    ax2.set_title("Two-Sample Q-Q Plot")
    ax2.set_xlabel("Series 1 Quantiles")
    ax2.set_ylabel("Series 2 Quantiles")
    
    plt.tight_layout()
    plt.show()
    