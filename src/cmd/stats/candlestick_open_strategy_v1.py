from loguru import logger
from base_open_strategy import BaseOpenStrategy
from trading_engine_types import OpenSignal
from typing import List
from rpc.playground_pb2 import TickDelta
import pandas as pd

class CandlestickOpenStrategy(BaseOpenStrategy):
    def __init__(self, playground, updateFrequency: str, min_max_window_in_hours):
        super().__init__(playground, updateFrequency, 0, 0, 0, 0, min_max_window_in_hours)

    def get_sl_shift(self):
        return 0
    
    def get_tp_shift(self):
        return 0
    
    def get_sl_buffer(self):
        return 0
    
    def get_tp_buffer(self):
        return 0
    
    def check_for_new_signal(self, ltf_data, htf_data):
        return None, None
    
    def tick(self, tick_delta: List[TickDelta]) -> List[OpenSignal]:
        new_candles = super().tick(tick_delta)
        
        ltf_data = pd.DataFrame(self.candles_5m)
        htf_data = pd.DataFrame(self.candles_1h)
        
        if self.feature_set is None:
            _, self.feature_set = self.check_for_new_signal(ltf_data, htf_data)
        
        open_signals = []
        for c in new_candles:
            try:
                self.update_price_feed(c)
            except Exception as e:
                logger.error(f"updating price feed: {e}")
                continue
            
            if c.bar.cdl_hammer:
                logger.info(f"new hammer - {c.period} @ {c}")
            
            if c.bar.cdl_doji_10_0_1:
                logger.info(f"new doji - {c.period} @ {c}")
            
        return open_signals