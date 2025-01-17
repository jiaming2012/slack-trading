from backtester_playground_client_grpc import BacktesterPlaygroundClient, RepositorySource
from google.protobuf.json_format import MessageToDict
from simple_base_strategy import SimpleBaseStrategy
from generate_signals import new_supertrend_momentum_signal_factory, add_supertrend_momentum_signal_feature_set_v2, add_supertrend_momentum_signal_target_set
from dateutil.relativedelta import relativedelta
from typing import List, Tuple
from rpc.playground_pb2 import Candle, TickDelta
from utils import fetch_polygon_stock_chart_aggregated_as_list
from collections import deque
from dataclasses import dataclass
from enum import Enum
import pandas as pd

class OpenSignalName(Enum):
    CROSS_ABOVE_20 = 1
    CROSS_BELOW_80 = 2
    
@dataclass
class OpenSignal:
    name: OpenSignalName
    date: pd.Timestamp
    max_price_prediction: float
    min_price_prediction: float
    max_price_prediction_std_dev: float
    min_price_prediction_std_dev: float
    
class SimpleOpenStrategy(SimpleBaseStrategy):
    def __init__(self, playground, model_training_period_in_months):
        super().__init__(playground)
        
        if not model_training_period_in_months:
            raise Exception("model_training_period_in_months is required")
        
        if model_training_period_in_months < 1:
            raise Exception("model_training_period_in_months must be greater than 1")
        
        historical_start_date, historical_end_date = self.get_previous_year_date_range()
        candles_5m = playground.fetch_candles_v2(300, historical_start_date, historical_end_date)
        candles_1h = playground.fetch_candles_v2(3600, historical_start_date, historical_end_date)
        
        candles_5m_dicts = [MessageToDict(candle, always_print_fields_with_no_presence=True, preserving_proto_field_name=True) for candle in candles_5m]
        candles_1h_dicts = [MessageToDict(candle, always_print_fields_with_no_presence=True, preserving_proto_field_name=True) for candle in candles_1h]
        
        self.candles_5m = deque(candles_5m_dicts, maxlen=len(candles_5m_dicts))
        self.candles_1h = deque(candles_1h_dicts, maxlen=len(candles_1h_dicts))  
        self.previous_month = None
        self.factory = None
        self.model_training_period_in_months = model_training_period_in_months
        self.feature_set = None
        self.min_max_window_in_hours = 4
        
    def is_new_month(self):
        current_month = self.playground.timestamp.month
        result = current_month != self.previous_month
        self.previous_month = current_month
        return result
    
    def get_previous_year_date_range(self) -> Tuple[pd.Timestamp, pd.Timestamp]:
        current_date = self.playground.timestamp
        first_day_of_current_month = current_date.replace(day=1)
        first_day_of_previous_month = first_day_of_current_month - relativedelta(months=12)
        
        start_date = first_day_of_previous_month
        end_date = first_day_of_current_month - relativedelta(days=1)
        
        return start_date, end_date
    
    def update_price_feed(self, new_candle: Candle):
         # Convert the Protocol Buffer message to a dictionary
        new_candle_dict = MessageToDict(new_candle.bar, always_print_fields_with_no_presence=True, preserving_proto_field_name=True)
        
        # Add the new field
        timestamp_utc = pd.Timestamp(new_candle.bar.datetime)
        new_candle_dict['date'] = timestamp_utc.tz_convert('America/New_York')
        
        if new_candle.period == 300:
            self.candles_5m.append(new_candle_dict)
        elif new_candle.period == 3600:
            self.candles_1h.append(new_candle_dict)
        else:
            raise Exception(f"Unsupported period: {new_candle})")
            
    def check_for_new_signal(self, ltf_data: pd.DataFrame, htf_data: pd.DataFrame) -> Tuple[OpenSignalName, pd.DataFrame]:
        data_set = None
        
        if len(ltf_data) > 0 and len(htf_data) > 0:
            data_set = add_supertrend_momentum_signal_feature_set_v2(ltf_data, htf_data)
            
            if data_set.iloc[-1]['stochrsi_cross_below_80'] and data_set.iloc[-1]['superD_htf_50_3'] == -1:
                return (OpenSignalName.CROSS_BELOW_80, data_set)
            
            if data_set.iloc[-1]['stochrsi_cross_above_20'] and data_set.iloc[-1]['superD_htf_50_3'] == 1:
                return (OpenSignalName.CROSS_ABOVE_20, data_set)
        
        return None, data_set
    
    def tick(self, tick_delta: List[TickDelta]) -> List[OpenSignal]:
        new_candles = []
        for delta in tick_delta:
            if hasattr(delta, 'new_candles'):
                for c in delta.new_candles:
                    new_candles.append(c)
                    
        ltf_data = pd.DataFrame(self.candles_5m)
        htf_data = pd.DataFrame(self.candles_1h)
        
        open_signals = []
        for c in new_candles:
            self.update_price_feed(c)
            
            if c.period == 300:
                open_signal, self.feature_set = self.check_for_new_signal(ltf_data, htf_data)
                if open_signal:
                    print(f"New signal: {open_signal.name}")
                    
                    if not self.factory:
                        print("Skipping signal creation: factory not initialized")
                        continue
                
                    formatted_feature_set = self.feature_set.iloc[[-1]][self.factory.feature_columns]
                    
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
                    
                    open_signals.append(
                        OpenSignal(
                            open_signal, 
                            date, 
                            max_price_prediction, 
                            min_price_prediction, 
                            self.factory.max_price_prediction_std_dev, 
                            self.factory.min_price_prediction_std_dev
                        )
                    )
                    
        if self.is_new_month():
            if self.feature_set is None:
                print("Skipping model training: feature set is empty")
                return open_signals
            
            print("-" * 40)
            print(f"New month: {self.playground.timestamp}")
            print("-" * 40)
            target_set = add_supertrend_momentum_signal_target_set(self.feature_set, self.min_max_window_in_hours)
            self.factory = new_supertrend_momentum_signal_factory(target_set)
                    
        return open_signals


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