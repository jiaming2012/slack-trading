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

class OptimizedOpenStrategy(BaseOpenStrategy):
    def is_new_month(self):
        current_month = self.playground.timestamp.month
        result = current_month != self.previous_month
        self.previous_month = current_month
        return result
    
    def get_optimized_hyperparameters(self, start_date_obj: datetime) -> dict:
        # opt_start_date = start_date_obj - relativedelta(months=1) CHANGE THIS BACK
        opt_start_date = start_date_obj - relativedelta(days=5)
        opt_stop_date = start_date_obj - relativedelta(days=1)
        opt_start_date_str = opt_start_date.strftime("%Y-%m-%d")
        opt_stop_date_str = opt_stop_date.strftime("%Y-%m-%d")
        
        os.environ['START_DATE'] = opt_start_date_str
        os.environ['STOP_DATE'] = opt_stop_date_str
        os.environ['OPEN_STRATEGY'] = 'simple_open_strategy_v1'
        
        optimizer = TradingEngineOptimizer(self.n_calls)
        optimizer.optimize()
        average_hyperparameters = optimizer.compute_average_hyperparameters(0.1)
        return average_hyperparameters

    def tick(self, tick_delta: List[TickDelta]) -> List[OpenSignal]:
        if self.is_new_month():
            self.strategy = self.new_optimized_open_strategy()
        
        return self.strategy.tick(tick_delta)
    
    def new_optimized_open_strategy(self) -> BaseOpenStrategy:
        start_date = self.playground.timestamp
        
        params = self.get_optimized_hyperparameters(start_date)
        
        sl_shift = params['sl_shift']
        tp_shift = params['tp_shift']
        sl_buffer = params['sl_buffer']
        tp_buffer = params['tp_buffer']
        min_max_window_in_hours = params['min_max_window_in_hours']
        
        return SimpleOpenStrategy(self.playground, self.updateFrequency, sl_shift=sl_shift, tp_shift=tp_shift, sl_buffer=sl_buffer, tp_buffer=tp_buffer, min_max_window_in_hours=min_max_window_in_hours) 
        
    def __init__(self, playground, updateFrequency, n_calls=10):
        super().__init__(playground)
        
        self.n_calls = n_calls
        self.previous_month = self.playground.timestamp.month
        self.updateFrequency = updateFrequency
        self.strategy = self.new_optimized_open_strategy()
        
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
    grpc_host = 'http://45.77.223.21'
    updateFrequency = 'weekly'
    n_calls = 10
    
    playground = BacktesterPlaygroundClient(balance, symbol, start_date, end_date, repository_source, csv_path, grpc_host=grpc_host)
    
    strategy = OptimizedOpenStrategy(playground, updateFrequency, n_calls)
    
    while not strategy.is_complete():
        strategy.tick()
        
    print("Done")