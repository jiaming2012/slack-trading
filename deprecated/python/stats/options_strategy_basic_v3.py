from datetime import datetime, time, timedelta
from typing import List, Tuple
from loguru import logger
from dateutil.parser import isoparse
from dataclasses import dataclass
import math
import numpy as np
from zoneinfo import ZoneInfo

import pandas as pd
from backtester_playground_client_grpc import BacktesterPlaygroundClient, Repository, RepositorySource, CreatePolygonPlaygroundRequest, PlaygroundEnvironment, OrderSide
from rpc.playground_pb2 import GetOptionsLadderRequest, OptionLadderContract
from base_open_strategy_v2 import BaseOpenStrategyV2
from trading_engine_types import OpenSignalV3, OpenSignalName
from rpc.playground_pb2 import Candle

# v3 uses the binomial american option pricing model to estimate expected profit of selling options

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
    
class OptionsStrategyBasic(BaseOpenStrategyV2):
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
    
    def __init__(self, playground, symbol: str, logger, sl_buffer=0.0, tp_buffer=0.0):
        sl_shift = 0.0
        tp_shift = 0.0
        
        super().__init__(playground, symbol, sl_shift, tp_shift, sl_buffer, tp_buffer)
        
        self.logger = logger.bind(symbol=symbol)
        self.symbol = symbol
        self.use_htf_data = True
        self.candles = []
    
    
    # Deprecated chatgpt version in favor of google version
    # def calculate_expected_profit_binomial_american(self, stock_price, strike_price, premium_received, T, r, N):
    #     """
    #     Calculates the expected profit of selling an American call option.
    #     Uses the Cox-Ross-Rubinstein (CRR) Binomial Pricing Model adapted for early exercise.

    #     Args:
    #         stock_price (float): Current stock price.
    #         strike_price (float): Strike price of the call option.
    #         premium_received (float): Premium received for selling the option today (user input).
    #         T (float): Time to expiration in years.
    #         r (float): Annualized risk-free interest rate.
    #         N (int): Number of time steps.

    #     Returns:
    #         float: The expected profit of selling the call option.
    #     """
        
    #     dt = T / N
    #     sigma = self.playground.stats.calculate_local_model_volatility(stock_price)
    #     u = math.exp(sigma * math.sqrt(dt))
    #     d = 1 / u
    #     p = (math.exp(r * dt) - d) / (u - d)

    #     # Initialize asset prices at maturity
    #     asset_prices = [0.0] * (N + 1)
    #     for i in range(N + 1):
    #         asset_prices[i] = stock_price * (u ** (N - i)) * (d ** i)

    #     # Initialize option values at maturity
    #     option_values = [0.0] * (N + 1)
    #     for i in range(N + 1):
    #         option_values[i] = max(0, asset_prices[i] - strike_price)

    #     # Backward induction for option price at earlier nodes
    #     for j in range(N - 1, -1, -1):
    #         for i in range(j + 1):
    #             option_values[i] = math.exp(-r * dt) * (p * option_values[i] + (1 - p) * option_values[i + 1])
    #             # Check for early exercise
    #             exercise_value = asset_prices[i] - strike_price
    #             option_values[i] = max(option_values[i], exercise_value)

    #     expected_profit = premium_received - option_values[0]
    #     return expected_profit
    
    
    def check_for_new_signal(self, new_candle: pd.DataFrame):
        st_direction = new_candle.superD_50_3
        previous_supertrend_count = self.candles_ltf_idx
        for i in range(self.candles_ltf_idx-1, 0, -1):
            previous_direction = self.candles_ltf.iloc[i].superD_50_3
            if previous_direction != st_direction:
                ltf_supertrend_count = abs(previous_supertrend_count - i)
                ltf_supertrend_value = new_candle.superT_50_3
                previous_supertrend_count = i

                signal = OpenSignalV4(
                    symbol = self.symbol,
                    name="LONG_OPTION_ENTRY",
                    timestamp=isoparse(new_candle.datetime),
                    price=new_candle.close,
                    ltf_supertrend_count=ltf_supertrend_count,
                    ltf_supertrend_value=ltf_supertrend_value,
                    ltf_supertrend_direction=st_direction,
                    expected_volatility=0.0,
                )
                
                signal.expected_volatility = self.playground.stats.calculate_local_model_volatility(signal)
                
                return signal
        
        return None
    
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
                               
    def tick(self, tick_delta) -> List[OpenSignalV4]:        
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
                    requested_price
                )
                
                self.logger.info(f"Closed assigned options by buying {abs(qty)} shares of {self.symbol} at market price.")

        # Check for new open signals
        open_signals = []
        new_candles: List[Candle] = tick_delta.new_candles if hasattr(tick_delta, 'new_candles') else []
        for c in new_candles:
            self.logger.trace(f"new candle - {c.period} @ {c.bar.datetime} - {c.bar.close}")
            
            if not c.period == self.playground.ltf_seconds:
                continue
            
            if not c.symbol == self.symbol:
                continue
            
            self.logger.trace(f"Processing LTF candle - {c.period} @ {c.bar.datetime} - {c.bar.close}")
                                        
            open_signal = self.check_for_new_signal(c.bar)
            if open_signal:
                open_signals.append(open_signal)

            self.append_candle(c.bar)


        return open_signals
    
def run(playground: BacktesterPlaygroundClient, symbol: str, logger):
    max_open_count = 3
    strategy = OptionsStrategyBasic(playground, symbol, logger)
    
    while not strategy.is_complete():
        tick_deltas = playground.flush_new_state_buffer()
        for tick_delta in tick_deltas:
            open_signals = strategy.tick(tick_delta)
            
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
                        logger.warning(f"No suitable option contract found for {signal.symbol} at price {signal.kwargs['current_price']}")
                        continue
                    
                    playground.place_order(
                        target_contract.symbol, 
                        1, 
                        OrderSide.SELL_TO_OPEN,
                        'option'
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
            candle_dt =isoparse(candle.datetime)
            
            candle_day_of_week = candle_dt.weekday()
            if candle_day_of_week == 4 and is_after_market_open(candle_dt):
                if candle.is_extended_hours:
                    close_candle = ltf_candles[j - 1]
            
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
    
    
if __name__ == "__main__":
    balance = 30000
    symbol = 'AMZN'
    start_date = '2025-08-01' # Dont start on holidays
    end_date = '2025-12-06'
    repository_source = RepositorySource.POLYGON
    csv_path = None
    twirp_host = 'http://127.0.0.1:5051'
    updateFrequency = 'daily'
    
    repos = OptionsStrategyBasic.get_repositories(symbol, datetime.fromisoformat(start_date), datetime.fromisoformat(end_date))
    
    req = CreatePolygonPlaygroundRequest(
        balance=balance,
        start_date=start_date,
        stop_date=end_date,
        repositories=repos,
        environment=PlaygroundEnvironment.SIMULATOR.value
    )
    
    live_account_type = None
    
    playground = BacktesterPlaygroundClient(req, live_account_type, repository_source, logger, twirp_host=twirp_host)
    
    generate_signal_stats(playground, symbol)
    
    playground.stats.generate_model()
    
    max_open_count = 3
    strategy = OptionsStrategyBasic(playground, symbol, logger)
        
    while not strategy.is_complete():
        tick_deltas = playground.flush_new_state_buffer()
        for tick_delta in tick_deltas:
            open_qty = playground.get_options_quantity(symbol)
            if abs(open_qty) >= max_open_count:
                continue
            
            open_signals = strategy.tick(tick_delta)
            
            for signal in open_signals:
                expiration_in_days = strategy.find_next_friday(signal.timestamp)
                
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

                        if profit_american > highest_expected_profit:
                            highest_expected_profit = profit_american
                            target_contract = c
                                                
                    if target_contract is None:
                        logger.warning(f"No suitable option contract found for {signal.symbol} at price {signal.price}")
                        continue
                    
                    # In order to calculate the option EV, we 
                    playground.place_order(
                        target_contract.symbol, 
                        1, 
                        OrderSide.SELL_TO_OPEN,
                        'option',
                        attributes={"ev": str(highest_expected_profit), "stock_price": str(signal.price)}
                    )
                    
                    logger.info(f"Open Signal: {signal.name} at {signal.timestamp} for {signal.symbol}")
                
        playground.tick(playground.ltf_seconds)
        

    logger.info(f"Done - playground id: {playground.id}")
    