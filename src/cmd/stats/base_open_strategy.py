from loguru import logger
from abc import ABC, abstractmethod
from rpc.playground_pb2 import Candle
import pandas as pd
from google.protobuf.json_format import MessageToDict
from dateutil.relativedelta import relativedelta
from datetime import datetime, timedelta
from collections import deque
from typing import List, Tuple
from rpc.playground_pb2 import TickDelta
from trading_engine_types import OpenSignal, OpenSignalName

class BaseOpenStrategy(ABC):
    def is_new_month(self):
        current_month = self.playground.timestamp.month
        result = current_month != self.previous_month
        self.previous_month = current_month
        return result
    
    def is_new_week(self):
        current_week = self.playground.timestamp.isocalendar().week
        result = current_week != self.previous_week
        self.previous_week = current_week
        return result
    
    def is_new_day(self):
        current_day = self.playground.timestamp.day
        result = current_day != self.previous_day
        self.previous_day = current_day
        return result
    
    def get_previous_year_date_range(self, period_in_seconds: int) -> Tuple[pd.Timestamp, pd.Timestamp]:
        current_date = self.playground.timestamp
        
        # Align the start time to the nearest period boundary
        aligned_start = current_date - timedelta(seconds=current_date.timestamp() % period_in_seconds)
        
        previous_year_end = aligned_start
        previous_year_start = aligned_start - relativedelta(months=12)
        
        return previous_year_start, previous_year_end
    
    def __init__(self, playground, updateFrequency: str, sl_shift=0.0, tp_shift=0.0, sl_buffer=0.0, tp_buffer=0.0, min_max_window_in_hours=4):
        self.playground = playground
        self.timestamp = playground.timestamp
        
        historical_start_date, historical_end_date = self.get_previous_year_date_range(300)
        candles_5m = playground.fetch_candles_v2(300, historical_start_date, historical_end_date)
        
        historical_start_date, historical_end_date = self.get_previous_year_date_range(3600)
        candles_1h = playground.fetch_candles_v2(3600, historical_start_date, historical_end_date)
        
        candles_5m_dicts = [MessageToDict(candle, always_print_fields_with_no_presence=True, preserving_proto_field_name=True) for candle in candles_5m]
        candles_1h_dicts = [MessageToDict(candle, always_print_fields_with_no_presence=True, preserving_proto_field_name=True) for candle in candles_1h]
        
        self.candles_5m = deque(candles_5m_dicts, maxlen=len(candles_5m_dicts))
        self.candles_1h = deque(candles_1h_dicts, maxlen=len(candles_1h_dicts)) 
    
        self.previous_month = None
        self.previous_week = None
        self.previous_day = None
        self.factory = None
        self.feature_set = None
        self.min_max_window_in_hours = min_max_window_in_hours
        self.sl_buffer = sl_buffer
        self.tp_buffer = tp_buffer
        self.sl_shift = sl_shift
        self.tp_shift = tp_shift
        
        if updateFrequency == 'daily':
            self.previous_day = self.playground.timestamp.day
            self.should_update_model = self.is_new_day
            self.update_model_reason = 'new day'
        elif updateFrequency == 'weekly':
            self.previous_week = self.playground.timestamp.isocalendar().week
            self.should_update_model = self.is_new_week
            self.update_model_reason = 'new week'
        elif updateFrequency == 'monthly':
            self.previous_month = self.playground.timestamp.month
            self.should_update_model = self.is_new_month
            self.update_model_reason = 'new month'
        else:
            raise Exception(f"Unsupported update frequency: {updateFrequency}")
        
    def is_complete(self):
        return self.playground.is_backtest_complete()
    
    def update_price_feed(self, new_candle: Candle):
         # Convert the Protocol Buffer message to a dictionary
        new_candle_dict = MessageToDict(new_candle.bar, always_print_fields_with_no_presence=True, preserving_proto_field_name=True)
        
        new_candle_timestamp_utc = pd.Timestamp(new_candle.bar.datetime)
        
        if new_candle.period == 300:
            prev_candle_timestamp_utc = pd.Timestamp(self.candles_5m[-1]['datetime'])
            
            # append only if sorted by timestamp
            if len(self.candles_5m) > 0 and prev_candle_timestamp_utc > new_candle_timestamp_utc:
                logger.error(f'{prev_candle_timestamp_utc} > {new_candle_timestamp_utc}')
                raise Exception("Candles (5m) are not sorted by timestamp")
            
            self.candles_5m.append(new_candle_dict)
        elif new_candle.period == 3600:
            prev_candle_timestamp_utc = pd.Timestamp(self.candles_1h[-1]['datetime'])
            
            # append only if sorted by timestamp
            if len(self.candles_1h) > 0 and prev_candle_timestamp_utc > new_candle_timestamp_utc:
                logger.error(f'{prev_candle_timestamp_utc} > {new_candle_timestamp_utc}')
                raise Exception("Candles (1h) are not sorted by timestamp")
            
            self.candles_1h.append(new_candle_dict)
        else:
            raise Exception(f"Unsupported period: {new_candle})")
    
    @abstractmethod        
    def get_sl_shift(self):
        pass
    
    @abstractmethod
    def get_tp_shift(self):
        pass
    
    @abstractmethod
    def get_sl_buffer(self):
        pass
    
    @abstractmethod
    def get_tp_buffer(self):
        pass
    
    @abstractmethod
    def check_for_new_signal(self, ltf_data: pd.DataFrame, htf_data: pd.DataFrame) -> Tuple[OpenSignalName, pd.DataFrame]:
        pass
    
    @abstractmethod
    def tick(self, tick_delta: List[TickDelta]) -> List[OpenSignal]:
        new_candles = []
        for delta in tick_delta:
            if hasattr(delta, 'new_candles'):
                for c in delta.new_candles:
                    new_candles.append(c)
            
        return new_candles