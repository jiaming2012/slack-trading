from datetime import datetime
from typing import List, Tuple
from loguru import logger

import pandas as pd
from backtester_playground_client_grpc import BacktesterPlaygroundClient, Repository, RepositorySource, CreatePolygonPlaygroundRequest, PlaygroundEnvironment
from base_open_strategy_v2 import BaseOpenStrategyV2
from trading_engine_types import OpenSignalV3, OpenSignalName
from rpc.playground_pb2 import Candle

class OptionsStrategyBasic(BaseOpenStrategyV2):
    @classmethod
    def get_repositories(cls, symbol: str, start_date: datetime, end_date: datetime) -> List[Repository]:
        htf_repo_daily = Repository(
            symbol=symbol,
            timespan_multiplier=1,
            timespan_unit='day',
            indicators=["supertrend", "atr"],
            history_in_days=365
        )

        
        return [ htf_repo_daily ]
    
    def __init__(self, playground, symbol: str, logger, sl_buffer=0.0, tp_buffer=0.0):
        sl_shift = 0.0
        tp_shift = 0.0
        
        super().__init__(playground, symbol, sl_shift, tp_shift, sl_buffer, tp_buffer)
        
        self.logger = logger.bind(symbol=symbol)
        self.symbol = symbol
        self.use_htf_data = True
    
    
    def get_max_per_trade_risk_percentage(self):
        return self.max_per_trade_risk_percentage
    
    def get_sl_shift(self):
        return self.sl_shift
    
    def get_tp_shift(self):
        return self.tp_shift
    
    def get_sl_buffer(self):
        return self.sl_buffer
    
    def get_tp_buffer(self):
        return self.tp_buffer
    
    def find_supertrend_start(self, df: pd.DataFrame) -> int:
        current_direction = df.iloc[-1]['superD_50_3']
        for i in range(-1, -1 * len(df), -1):
            previous_direction = df.iloc[i]['superD_50_3']
            if previous_direction != current_direction:
                return i
            
        return -1
    
    # def bars_overlap(self, bar1: SignalBar, bar2: SignalBar) -> bool:
    #     # Get high and low of each bar
    #     low1, high1 = min(bar1.open, bar1.close), max(bar1.open, bar1.close)
    #     low2, high2 = min(bar2.open, bar2.close), max(bar2.open, bar2.close)

    #     # Check for overlap
    #     return not (high1 <= low2 or high2 <= low1)
    
    # def signal_constraint(self, newBar: SignalBar, pastSignalBars: List[SignalBar]) -> bool:
    #     for b in pastSignalBars:
    #         if self.bars_overlap(newBar, b):
    #             logger.debug(f"Signal constraint violated: {newBar} intersects with {b}")
    #             return True
            
    #     return False

    def check_for_new_signal(self, ltf_data: pd.DataFrame, htf_data: pd.DataFrame, htf_data_daily: pd.DataFrame, htf_data_weekly: pd.DataFrame, open_trade_count: int) -> Tuple[OpenSignalName, pd.DataFrame, dict]:
        data_set = None
                           
        return None, data_set, None
    
    def tick(self, tick_delta) -> List[OpenSignalV3]:        
        ltf_data = pd.DataFrame(self.candles_ltf)
        new_candles: List[Candle] = tick_delta.new_candles if hasattr(tick_delta, 'new_candles') else []
        open_signals = []
        for c in new_candles:
            self.logger.trace(f"new candle - {c.period} @ {c.bar.datetime} - {c.bar.close}")
            
            if c.period == self.playground.ltf_seconds:
                symbol = self.symbol                
                    
        return open_signals

if __name__ == "__main__":
    balance = 30000
    symbol = 'AAPL'
    start_date = '2025-04-16'
    end_date = '2025-11-20'
    repository_source = RepositorySource.POLYGON
    csv_path = None
    twirp_host = 'http://127.0.0.1:5051'
    updateFrequency = 'daily'
    
    repos = OptionsStrategyBasic.get_repositories(symbol, datetime.fromisoformat(start_date), datetime.fromisoformat(end_date))
    
    req = CreatePolygonPlaygroundRequest(
        balance=balance,
        start_date=start_date,
        stop_date=end_date,
        repositories=repos,
        environment=PlaygroundEnvironment.SIMULATOR.value
    )
    
    live_account_type = None
    
    playground = BacktesterPlaygroundClient(req, live_account_type, repository_source, logger, twirp_host=twirp_host)
    additional_profit_risk_percentage = 0.0
    max_open_count = 3
    strategy = OptionsStrategyBasic(playground, symbol, logger)
    
    while not strategy.is_complete():
        tick_deltas = playground.flush_new_state_buffer()
        for tick_delta in tick_deltas:
            strategy.tick(tick_delta)
        playground.tick(playground.ltf_seconds)
        

    logger.info("Done")