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
from engine.types import OpenSignal, OpenSignalV3, OpenSignalName

class BaseOpenStrategyV2(ABC):
    def is_complete(self) -> bool:
        return self.playground.is_backtest_complete()
    
    def get_previous_year_date_range(self, period_in_seconds: int) -> Tuple[pd.Timestamp, pd.Timestamp]:
        current_date = self.playground.timestamp
        
        # Align the start time to the nearest period boundary
        aligned_start = current_date - timedelta(seconds=current_date.timestamp() % period_in_seconds)
        
        previous_year_end = aligned_start
        previous_year_start = aligned_start - relativedelta(months=12)
        
        return previous_year_start, previous_year_end
    
    def append_candle(self, candle: Candle):
        if self.candles_ltf_idx >= len(self.candles_ltf):
            new_size = len(self.candles_ltf) * 2
            self.candles_ltf = self.candles_ltf.reindex(range(new_size))
            
        self.candles_ltf.iloc[self.candles_ltf_idx] = MessageToDict(candle, always_print_fields_with_no_presence=True, preserving_proto_field_name=True)
        self.candles_ltf_idx += 1
            
    def __init__(self, playground, symbol, sl_shift=0.0, tp_shift=0.0, sl_buffer=0.0, tp_buffer=0.0):
        if type(symbol) is not str:
            raise Exception(f"Symbol must be a string, got {type(symbol)}")
        
        self.playground = playground
        self.timestamp = playground.timestamp
        self.symbol = symbol
        
        historical_start_date_ltf, historical_end_date_ltf = self.get_previous_year_date_range(playground.ltf_seconds)
        candles_ltf = playground.fetch_candles_v2(symbol, playground.ltf_seconds, historical_start_date_ltf, historical_end_date_ltf)
        
        if len(candles_ltf) == 0:
            raise Exception(f"No LTF candles found for symbol {symbol} from {historical_start_date_ltf} to {historical_end_date_ltf}")
        
        candles_ltf_dict = MessageToDict(candles_ltf[0], always_print_fields_with_no_presence=True, preserving_proto_field_name=True)
        
        self.candles_ltf = pd.DataFrame(index=range(len(candles_ltf) * 2), columns=candles_ltf_dict.keys())
        self.candles_ltf_idx = 0
        for c in candles_ltf:
            self.append_candle(c)
        # self.candles_ltf = deque(candles_ltf_dicts, maxlen=len(candles_ltf_dicts))
            
        logger.info(f"Loaded {len(self.candles_ltf)} LTF candles", timestamp=self.timestamp, trading_operation=None)

        self.sl_buffer = sl_buffer
        self.tp_buffer = tp_buffer
        self.sl_shift = sl_shift
        self.tp_shift = tp_shift
        self.symbol = None
    
    def update_price_feed(self, new_candles: List[Candle]) -> None:
        # new_candles = self.parse_new_candles(tick_delta)
        
        for new_candle in new_candles:
            # Convert the Protocol Buffer message to a dictionary
            new_candle_dict = MessageToDict(new_candle.bar, always_print_fields_with_no_presence=True, preserving_proto_field_name=True)
            
            new_candle_timestamp_utc = pd.Timestamp(new_candle.bar.datetime)
            
            if new_candle.period == self.playground.ltf_seconds:
                prev_candle_timestamp_utc = pd.Timestamp(self.candles_ltf[-1]['datetime'])
                
                # append only if sorted by timestamp
                if len(self.candles_ltf) > 0 and prev_candle_timestamp_utc > new_candle_timestamp_utc:
                    logger.error(f'{prev_candle_timestamp_utc} > {new_candle_timestamp_utc}', timestamp=self.timestamp, trading_operation=None)
                    raise Exception("Candles (5m) are not sorted by timestamp")
                
                self.candles_ltf.append(new_candle_dict)
          
            else:
                raise Exception(f"Unsupported period: {new_candle})")
            
        return
    
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
    def tick(self, new_candles: List[Candle]) -> List[OpenSignalV3]:
        pass
        
    def parse_new_candles(self, tick_delta: List[TickDelta]) -> List[Candle]:
        new_candles = []
        for delta in tick_delta:
            if hasattr(delta, 'new_candles'):
                for c in delta.new_candles:
                    if c.symbol != self.symbol:
                        logger.error(f"Received candle for different symbol: {c.symbol} != {self.symbol}", timestamp=self.timestamp, trading_operation=None)
                        continue
                    
                    new_candles.append(c)
            
        return new_candles
            
