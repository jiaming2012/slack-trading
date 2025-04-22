from loguru import logger
from backtester_playground_client_grpc import BacktesterPlaygroundClient, RepositorySource, CreatePolygonPlaygroundRequest, Repository, PlaygroundEnvironment, OrderSide
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
from trading_engine_types import OpenSignalV3, OpenSignalName

@dataclass
class SignalBar:
    open: float
    close: float

# V4: Targets a specific risk/reward
class SimpleStackOpenStrategyV1(BaseOpenStrategy):
    def __init__(self, playground, max_open_count, max_per_trade_risk_percentage, additional_profit_risk_percentage, symbol: str, logger, sl_buffer=0.0, tp_buffer=0.0):
        sl_shift = 0.0
        tp_shift = 0.0
        
        super().__init__(playground, sl_shift, tp_shift, sl_buffer, tp_buffer)
        
        self.logger = logger.bind(symbol=symbol)
        self.additional_profit_risk_percentage = additional_profit_risk_percentage
        self.max_open_count = max_open_count
        self.max_per_trade_risk_percentage = max_per_trade_risk_percentage
        self.factory_meta = {}
    
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
    
    def bars_overlap(self, bar1: SignalBar, bar2: SignalBar) -> bool:
        # Get high and low of each bar
        low1, high1 = min(bar1.open, bar1.close), max(bar1.open, bar1.close)
        low2, high2 = min(bar2.open, bar2.close), max(bar2.open, bar2.close)

        # Check for overlap
        return not (high1 <= low2 or high2 <= low1)
    
    def signal_constraint(self, newBar: SignalBar, pastSignalBars: List[SignalBar]) -> bool:
        for b in pastSignalBars:
            if self.bars_overlap(newBar, b):
                logger.debug(f"Signal constraint violated: {newBar} intersects with {b}")
                return True
            
        return False

    def check_for_new_signal(self, ltf_data: pd.DataFrame, htf_data: pd.DataFrame) -> Tuple[OpenSignalName, pd.DataFrame, dict]:
        data_set = None
        
        if len(ltf_data) > 0 and len(htf_data) > 0:
            data_set = add_supertrend_momentum_signal_feature_set_v2(ltf_data, htf_data)
            
            start_index = self.find_supertrend_start(data_set)
            
            past_signal_bars: List[SignalBar] = []

            for i in range(start_index, 0, 1):
                if data_set.iloc[i]['superD_50_3'] == 1:
                    signal_criteria = data_set.iloc[i]['close'] < data_set.iloc[i]['open']
                    side = OrderSide.BUY
                    sl_buffer = self.sl_buffer * -1
                elif data_set.iloc[i]['superD_50_3'] == -1:
                    signal_criteria = data_set.iloc[i]['close'] > data_set.iloc[i]['open']
                    side = OrderSide.SELL_SHORT
                    sl_buffer = self.sl_buffer
                else:
                    signal_criteria = False
                
                if len(past_signal_bars) >= self.max_open_count:
                    logger.trace(f"Signal constraint: {past_signal_bars} already met for current supertrend", trading_operation="check_for_new_signal", timestamp=self.playground.timestamp)
                    break    
                
                if signal_criteria:
                    new_bar = SignalBar(data_set.iloc[i]['open'], data_set.iloc[i]['close'])
                    if not self.signal_constraint(new_bar, past_signal_bars):
                        past_signal_bars.append(new_bar)
                        
                        if i == -1:
                            sl = data_set.iloc[i]['superT_50_3'] + sl_buffer
                            logger.info(f"[LIVE] Signal criteria met at index {i}: {data_set.iloc[i]['date']}, with sl: {sl}", trading_operation="check_for_new_signal", timestamp=self.playground.timestamp)
                            return OpenSignalName.SUPERTREND_STACK_SIGNAL, data_set, { 'count': len(past_signal_bars), 'sl': sl, 'side': side }
                        else:
                            logger.debug(f"[PAST] Signal criteria met at index {i}: {data_set.iloc[i]['date']}", trading_operation="check_for_new_signal", timestamp=self.playground.timestamp)
                            
        
        return None, data_set, None
    
    def tick(self, new_candles: List[Candle]) -> List[OpenSignalV3]:
        ltf_data = pd.DataFrame(self.candles_ltf)
        htf_data = pd.DataFrame(self.candles_htf)
        
        open_signals = []
        for c in new_candles:
            # todo: move this to a debug log. Move other debug logs to trace.
            self.logger.trace(f"new candle - {c.period} @ {c.bar.datetime} - {c.bar.close}")
            
            if c.period == self.playground.ltf_seconds:
                open_signal, self.feature_set, kwargs = self.check_for_new_signal(ltf_data, htf_data)
                if open_signal:
                    self.logger.debug(f"new signal: {open_signal.name}: {kwargs}")
                    
                    timestamp_utc = pd.Timestamp(c.bar.datetime)
                    date = timestamp_utc.tz_convert('America/New_York')
                    
                    realized_profit = self.playground.get_realized_profit()
                    self.logger.trace(f"Realized profit: {realized_profit}")
                    
                    symbol = self.playground.symbol
                    open_trade_count = len(self.playground.fetch_open_orders(symbol))
                    self.logger.trace(f"Open trade count: {open_trade_count}")
                    
                    additional_equity_risk = 0
                    if realized_profit > 0 and open_trade_count < 2:
                        additional_equity_risk = realized_profit * self.additional_profit_risk_percentage
                    
                    open_signals.append(
                        OpenSignalV3(
                            open_signal, 
                            date, 
                            kwargs,
                            additional_equity_risk=additional_equity_risk,
                        )
                    )
                    
        return open_signals


if __name__ == "__main__":
    balance = 10000
    symbol = 'AAPL'
    start_date = '2024-10-13'
    end_date = '2024-11-10'
    repository_source = RepositorySource.POLYGON
    csv_path = None
    twirp_host = 'http://45.77.223.21'
    updateFrequency = 'weekly'
    
    htf_repo = Repository(
        symbol=symbol,
        timespan_multiplier=60,
        timespan_unit='minute',
        indicators=["supertrend"],
        history_in_days=365
    )
    
    ltf_repo = Repository(
            symbol=symbol,
            timespan_multiplier=15,
            timespan_unit='minute',
            indicators=["supertrend", "stochrsi", "moving_averages", "lag_features", "atr", "stochrsi_cross_above_20", "stochrsi_cross_below_80"],
            history_in_days=10
        )
    
    req = CreatePolygonPlaygroundRequest(
        balance=balance,
        start_date=start_date,
        stop_date=end_date,
        repositories=[ltf_repo, htf_repo],
        environment=PlaygroundEnvironment.SIMULATOR.value
    )
    
    live_account_type = None
    
    playground = BacktesterPlaygroundClient(req, live_account_type, repository_source, logger, twirp_host=twirp_host)
    additional_profit_risk_percentage = 0.0
    max_open_count = 3
    strategy = SimpleStackOpenStrategyV1(playground, max_open_count, additional_profit_risk_percentage, updateFrequency, symbol, logger)
    
    while not strategy.is_complete():
        tick_delta = playground.flush_new_state_buffer()
        strategy.tick(tick_delta)
        playground.tick(playground.ltf_seconds)
        
    logger.info("Done")