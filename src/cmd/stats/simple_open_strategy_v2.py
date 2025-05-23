from loguru import logger
from backtester_playground_client_grpc import BacktesterPlaygroundClient, RepositorySource
from simple_open_strategy_v1 import SimpleOpenStrategy
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
from trading_engine_optimizer import TradingEngineOptimizer
import os 

class SimpleOptimizedOpenStrategy(BaseOpenStrategy):
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
    
    def get_optimized_hyperparameters(self, start_date_obj: datetime) -> dict:
        opt_start_date = start_date_obj - relativedelta(weeks=1)
        opt_stop_date = start_date_obj - relativedelta(days=1)
        opt_start_date_str = opt_start_date.strftime("%Y-%m-%d")
        opt_stop_date_str = opt_stop_date.strftime("%Y-%m-%d")
        
        os.environ['START_DATE'] = opt_start_date_str
        os.environ['STOP_DATE'] = opt_stop_date_str
        os.environ['OPEN_STRATEGY'] = 'simple_open_strategy_v1'
        
        logger.info(f"Optimizing hyperparameters from {opt_start_date_str} to {opt_stop_date_str}")
        
        optimizer = TradingEngineOptimizer(self.n_calls)
        optimizer.optimize()
        average_hyperparameters = optimizer.compute_average_hyperparameters(0.1)
        return average_hyperparameters

    def tick(self, tick_delta: List[TickDelta]) -> List[OpenSignal]:
        if self.should_update_optimizer():
            self.strategy = self.new_optimized_open_strategy()
        
        return self.strategy.tick(tick_delta)
    
    def new_optimized_open_strategy(self) -> BaseOpenStrategy:
        start_date = self.playground.timestamp
        
        params = self.get_optimized_hyperparameters(start_date)
        
        sl_shift = round(params['sl_shift'], 2)
        tp_shift = round(params['tp_shift'], 2)
        sl_buffer = round(params['sl_buffer'], 2)
        tp_buffer = round(params['tp_buffer'], 2)
        min_max_window_in_hours = params['min_max_window_in_hours']
        
        logger.success(f"Optimized hyperparameters: sl_shift={sl_shift}, tp_shift={tp_shift}, sl_buffer={sl_buffer}, tp_buffer={tp_buffer}, min_max_window_in_hours={min_max_window_in_hours}")
        
        return SimpleOpenStrategy(self.playground, self.updateModelFrequency, sl_shift=sl_shift, tp_shift=tp_shift, sl_buffer=sl_buffer, tp_buffer=tp_buffer, min_max_window_in_hours=min_max_window_in_hours) 
        
    def __init__(self, playground, updateModelFrequency, updateOptimizerFrequency, n_calls=10):
        if updateModelFrequency is None:
            raise ValueError("Environment variable MODEL_UPDATE_FREQUENCY is not set")
        
        super().__init__(playground)
        
        self.n_calls = n_calls
        self.previous_month = self.playground.timestamp.month
        self.previous_week = self.playground.timestamp.isocalendar().week
        self.previous_day = self.playground.timestamp.day
        self.updateModelFrequency = updateModelFrequency
        self.strategy = self.new_optimized_open_strategy()
        
        logger.info(f"updateOptimizerFrequency: {updateOptimizerFrequency}, n_calls: {n_calls}")
        
        if updateOptimizerFrequency == 'monthly':
            self.should_update_optimizer = self.is_new_month
        elif updateOptimizerFrequency == 'weekly':
            self.should_update_optimizer = self.is_new_week
        elif updateOptimizerFrequency == 'daily':
            self.should_update_optimizer = self.is_new_day
        else:
            raise ValueError(f"Invalid updateOptimizerFrequency: {updateOptimizerFrequency}")
        
    def get_sl_shift(self):
        return self.strategy.get_sl_shift()
    
    def get_tp_shift(self):
        return self.strategy.get_tp_shift()
    
    def get_sl_buffer(self):
        return self.strategy.get_sl_buffer()
    
    def get_tp_buffer(self):
        return self.strategy.get_tp_buffer()

if __name__ == "__main__":
    balance = 10000
    symbol = 'AAPL'
    start_date = '2024-10-10'
    end_date = '2024-11-10'
    repository_source = RepositorySource.POLYGON
    csv_path = None
    twirp_host = 'http://45.77.223.21'
    updateFrequency = 'weekly'
    n_calls = 10
    
    playground = BacktesterPlaygroundClient(balance, symbol, start_date, end_date, repository_source, csv_path, twirp_host=twirp_host)
    
    strategy = SimpleOptimizedOpenStrategy(playground, updateFrequency, n_calls)
    
    while not strategy.is_complete():
        strategy.tick()
        
    logger.info("Done")