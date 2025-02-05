from loguru import logger
from backtester_playground_client_grpc import BacktesterPlaygroundClient, RepositorySource
from google.protobuf.json_format import MessageToDict
from base_open_strategy import BaseOpenStrategy
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
    
class SimpleOpenStrategy(BaseOpenStrategy):
    def __init__(self, playground, updateFrequency: str, sl_shift=0.0, tp_shift=0.0, sl_buffer=0.0, tp_buffer=0.0, min_max_window_in_hours=4):
        super().__init__(playground)
        
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
    
    def get_sl_shift(self):
        return self.sl_shift
    
    def get_tp_shift(self):
        return self.tp_shift
    
    def get_sl_buffer(self):
        return self.sl_buffer
    
    def get_tp_buffer(self):
        return self.tp_buffer
    
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
        
        if self.feature_set is None:
            _, self.feature_set = self.check_for_new_signal(ltf_data, htf_data)
        
        open_signals = []
        for c in new_candles:
            try:
                self.update_price_feed(c)
            except Exception as e:
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
                    date = timestamp_utc.tz_convert('America/New_York')
                    logger.debug(f"Date: {date}")
                    logger.debug(f"Current bar close: {c.bar.close}")
                    logger.debug(f"Max price prediction: {max_price_prediction}")
                    logger.debug(f"Min price prediction: {min_price_prediction}")
                    logger.debug("-" * 40)
                    
                    open_signals.append(
                        OpenSignal(
                            open_signal, 
                            date, 
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
    grpc_host = 'http://localhost:5051'
    updateFrequency = 'weekly'
    
    playground = BacktesterPlaygroundClient(balance, symbol, start_date, end_date, repository_source, csv_path, grpc_host=grpc_host)
    
    strategy = SimpleOpenStrategy(playground, updateFrequency)
    
    while not strategy.is_complete():
        strategy.tick()
        
    logger.info("Done")