from backtester_playground_client_grpc import BacktesterPlaygroundClient, OrderSide, RepositorySource, PlaygroundNotFoundException
from generate_signals import new_supertrend_momentum_signal_factory
from dateutil.relativedelta import relativedelta
from typing import List, Dict
from rpc.playground_pb2 import TickDelta, Candle
from utils import fetch_polygon_stock_chart_aggregated_as_list
from collections import deque
import pandas as pd

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
    
    def get_previous_one_month_date_range(self):
        current_date = self.playground.timestamp
        first_day_of_current_month = current_date.replace(day=1)
        first_day_of_previous_month = first_day_of_current_month - relativedelta(months=1)
        
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
        
        pass
    
    def check_for_new_signal(self):
        pass
    
    def tick(self):
        self.playground.tick(self._tick_in_seconds)
        
        if self.is_new_month():
            print(f"New month: {self.playground.timestamp}")
            start_date, end_date = self.get_previous_one_month_date_range()
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
                self.check_for_new_signal()
            
        if self.factory:
            pass


if __name__ == "__main__":
    balance = 10000
    symbol = 'AAPL'
    start_date = '2024-06-03'
    end_date = '2024-09-30'
    repository_source = RepositorySource.POLYGON
    csv_path = None
    grpc_host = 'http://localhost:5051'
    
    playground = BacktesterPlaygroundClient(balance, symbol, start_date, end_date, repository_source, csv_path, grpc_host=grpc_host)
    
    strategy = SimpleOpenStrategy(playground)
    
    while not strategy.is_complete():
        strategy.tick()
        
    print("Done")