from loguru import logger
from backtester_playground_client_grpc import BacktesterPlaygroundClient, RepositorySource
from google.protobuf.json_format import MessageToDict
from base_open_strategy import BaseSimpleOpenStrategy
from generate_signals import new_supertrend_momentum_signal_factory, add_supertrend_momentum_signal_feature_set_v2, add_supertrend_momentum_signal_target_set
from dateutil.relativedelta import relativedelta
from typing import List, Tuple
from rpc.playground_pb2 import Candle, TickDelta
from utils import fetch_polygon_stock_chart_aggregated_as_list
from collections import deque
from dataclasses import dataclass
from datetime import datetime, timedelta
from enum import Enum
import pandas as pd
from trading_engine_types import OpenSignal, OpenSignalName
    
class SimpleOpenStrategy(BaseSimpleOpenStrategy):
    def __init__(self, playground, modelUpdateFrequency: str, sl_shift=0.0, tp_shift=0.0, sl_buffer=0.0, tp_buffer=0.0, min_max_window_in_hours=4):
        if modelUpdateFrequency is None:
            raise ValueError("Environment variable MODEL_UPDATE_FREQUENCY is not set")
        
        super().__init__(playground, modelUpdateFrequency, sl_shift, tp_shift, sl_buffer, tp_buffer, min_max_window_in_hours)
        
    def get_sl_shift(self):
        return self.sl_shift
    
    def get_tp_shift(self):
        return self.tp_shift
    
    def get_sl_buffer(self):
        return self.sl_buffer
    
    def get_tp_buffer(self):
        return self.tp_buffer
            
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
        new_candles = super().tick(tick_delta)
        
        ltf_data = pd.DataFrame(self.candles_ltf)
        htf_data = pd.DataFrame(self.candles_htf)
                    
        if self.feature_set is None:
            _, self.feature_set = self.check_for_new_signal(ltf_data, htf_data)
        
        open_signals = []
        for c in new_candles:
            try:
                self.update_price_feed(c)
            except Exception as e:
                if str(e) == "Candles (5m) are not sorted by timestamp":
                    print("Caught the expected exception:", e)
                    
                elif str(e) == "Candles (1h) are not sorted by timestamp":
                    print("Caught the expected exception:", e)
                    
                else:
                    logger.error(f"updating price feed: {e}")
                    continue
            
            # todo: move this to a debug log. Move other debug logs to trace.
            logger.trace(f"new candle - {c.period} @ {c.bar.datetime} - {c.bar.close}")
            
            if c.period == 300:
                open_signal, self.feature_set = self.check_for_new_signal(ltf_data, htf_data)
                if open_signal:
                    logger.debug(f"new signal: {open_signal.name}")
                    
                    if not self.factory:
                        logger.debug("Skipping signal creation: factory not initialized")
                        continue
                
                    formatted_feature_set = self.feature_set.iloc[[-1]][self.factory.feature_columns]
                    
                    max_price_prediction = self.factory.models['max_price_prediction'].predict(formatted_feature_set)[0]
                    min_price_prediction = self.factory.models['min_price_prediction'].predict(formatted_feature_set)[0]
                    
                    timestamp_utc = pd.Timestamp(c.bar.datetime)
                    tstamp = timestamp_utc.tz_convert('America/New_York')
                    logger.debug(f"Date: {tstamp}")
                    logger.debug(f"Current bar close: {c.bar.close}")
                    logger.debug(f"Max price prediction: {max_price_prediction}")
                    logger.debug(f"Min price prediction: {min_price_prediction}")
                    logger.debug("-" * 40)
                    
                    open_signals.append(
                        OpenSignal(
                            open_signal, 
                            tstamp, 
                            max_price_prediction, 
                            min_price_prediction,
                        )
                    )
                    
        if self.should_update_model() or self.factory is None:
            if self.feature_set is None:
                logger.debug("Skipping model training: feature set is empty")
                return open_signals
            
            if self.factory is None:
                logger.info(f"initializing factory @ {self.playground.timestamp}")
            else:
                logger.info(f"reinitializing factory for {self.update_model_reason} @ {self.playground.timestamp}")
                
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
    twirp_host = 'http://localhost:5051'
    updateFrequency = 'weekly'
    
    playground = BacktesterPlaygroundClient(balance, symbol, start_date, end_date, repository_source, csv_path, twirp_host=twirp_host)
    
    strategy = SimpleOpenStrategy(playground, updateFrequency)
    
    while not strategy.is_complete():
        strategy.tick()
        
    logger.info("Done")