from backtester_playground_client_grpc import BacktesterPlaygroundClient, OrderSide, RepositorySource, PlaygroundNotFoundException
from generate_signals import new_supertrend_momentum_signal_factory, add_supertrend_momentum_signal_feature_set
from dateutil.relativedelta import relativedelta
from typing import List, Tuple
from rpc.playground_pb2 import TickDelta, Candle
from utils import fetch_polygon_stock_chart_aggregated_as_list
from collections import deque
from enum import Enum
import pandas as pd

class OpenSignal(Enum):
    CROSS_ABOVE_20 = 1
    CROSS_BELOW_80 = 2

class BaseStrategy:
    def __init__(self, playground):
        self.playground = playground
        
        self.playground.tick(0)

        self.timestamp = playground.timestamp
        
    def is_complete(self):
        return self.playground.is_backtest_complete()
        
    def tick(self):
        raise Exception("Not implemented")
    
class SimpleOpenStrategy(BaseStrategy):
    def __init__(self, playground, tick_in_seconds=300):
        super().__init__(playground)
        
        self._tick_in_seconds = tick_in_seconds
        self.previous_month = None
        self.factory = None
        
        end_date = self.playground.timestamp - relativedelta(days=1)
        start_date = end_date - relativedelta(months=1)
        
        m5_rows = fetch_polygon_stock_chart_aggregated_as_list(self.playground.symbol, 5, 'minute', start_date, end_date)
        h1_rows = fetch_polygon_stock_chart_aggregated_as_list(self.playground.symbol, 1, 'hour', start_date, end_date)
        self.candles_5m = deque(m5_rows, maxlen=len(m5_rows))
        self.candles_1h = deque(h1_rows, maxlen=len(h1_rows))
        
                
    def is_new_month(self):
        current_month = self.playground.timestamp.month
        result = current_month != self.previous_month
        self.previous_month = current_month
        return result
    
    def get_previous_months_date_range(self, months):
        current_date = self.playground.timestamp
        first_day_of_current_month = current_date.replace(day=1)
        first_day_of_previous_month = first_day_of_current_month - relativedelta(months=months)
        
        start_date = first_day_of_previous_month
        end_date = first_day_of_current_month - relativedelta(days=1)
        
        return start_date, end_date
    
    def create_rolling_window(ltf_data, htf_data, start_date, end_date): 
        pass
    
    def update_price_feed(self, new_candle: Candle):
        timestamp_utc = pd.Timestamp(new_candle.bar.datetime)

        bar = {
            'Date': timestamp_utc.tz_convert('America/New_York'),
            'Open': new_candle.bar.open,
            'High': new_candle.bar.high,
            'Low': new_candle.bar.low,
            'Close': new_candle.bar.close,
            'Volume': new_candle.bar.volume
        }
        
        if new_candle.period == 300:
            self.candles_5m.append(bar)
        elif new_candle.period == 3600:
            self.candles_1h.append(bar)
        else:
            raise Exception(f"Unsupported period: {new_candle})")
            
    def check_for_new_signal(self) -> Tuple[OpenSignal, pd.DataFrame]:
        ltf_data = pd.DataFrame(self.candles_5m)
        htf_data = pd.DataFrame(self.candles_1h)
        
        data_set = add_supertrend_momentum_signal_feature_set(ltf_data, htf_data)
        if data_set.iloc[-1]['cross_below_80']:
            return (OpenSignal.CROSS_BELOW_80, data_set)
        
        if data_set.iloc[-1]['cross_above_20']:
            return (OpenSignal.CROSS_ABOVE_20, data_set)
        
        return None, data_set
    
    def tick(self):
        self.playground.tick(self._tick_in_seconds)
        
        if self.is_new_month():
            print("-" * 40)
            print(f"New month: {self.playground.timestamp}")
            print("-" * 40)
            start_date, end_date = self.get_previous_months_date_range(12)
            self.factory = new_supertrend_momentum_signal_factory(self.playground.symbol, start_date, end_date)
        
        tick_delta: List[TickDelta] = self.playground.flush_tick_delta_buffer()
        new_candles = None
        for delta in tick_delta:
            if hasattr(delta, 'new_candles'):
                new_candles = delta.new_candles
                break
        
        for c in new_candles:
            self.update_price_feed(c)
            
            if c.period == 300:
                open_signal, feature_set = self.check_for_new_signal()
                if open_signal:
                    print(f"New signal: {open_signal.name}")
                    
                    if not self.factory:
                        print("Skipping signal creation: factory not initialized")
                        continue
                
                    formatted_feature_set = feature_set.iloc[[-1]][self.factory.feature_columns]
                    
                    max_price_prediction = self.factory.models['max_price_prediction'].predict(formatted_feature_set)[0]
                    min_price_prediction = self.factory.models['min_price_prediction'].predict(formatted_feature_set)[0]
                    
                    timestamp_utc = pd.Timestamp(c.bar.datetime)
                    date = timestamp_utc.tz_convert('America/New_York')
                    print(f"Date: {date}")
                    print(f"Current bar close: {c.bar.close}")
                    print(f"Max price prediction: {max_price_prediction}")
                    print(f"Min price prediction: {min_price_prediction}")
                    print(f"Max price standard deviation: {self.factory.max_price_prediction_std_dev}")
                    print(f"Min price standard deviation: {self.factory.min_price_prediction_std_dev}")
                    print("-" * 40)


if __name__ == "__main__":
    balance = 10000
    symbol = 'AAPL'
    start_date = '2024-10-10'
    end_date = '2024-11-10'
    repository_source = RepositorySource.POLYGON
    csv_path = None
    grpc_host = 'http://localhost:5051'
    
    playground = BacktesterPlaygroundClient(balance, symbol, start_date, end_date, repository_source, csv_path, grpc_host=grpc_host)
    
    strategy = SimpleOpenStrategy(playground)
    
    while not strategy.is_complete():
        strategy.tick()
        
    print("Done")