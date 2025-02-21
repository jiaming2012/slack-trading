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
from trading_engine_types import OpenSignalV2, OpenSignalName
    
# V4: Targets a specific risk/reward
class SimpleOpenStrategyV4(BaseOpenStrategy):
    def __init__(self, playground, additional_profit_risk_percentage, updateFrequency: str, sl_shift=0.0, tp_shift=0.0, sl_buffer=0.0, tp_buffer=0.0, min_max_window_in_hours=4):
        super().__init__(playground, updateFrequency, sl_shift, tp_shift, sl_buffer, tp_buffer, min_max_window_in_hours)
        
        self.additional_profit_risk_percentage = additional_profit_risk_percentage
    
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
    
    def tick(self, tick_delta: List[TickDelta]) -> List[OpenSignalV2]:
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
                    logger.trace(f"Date: {date}")
                    logger.trace(f"Current bar close: {c.bar.close}")
                    logger.trace(f"Max price prediction: {max_price_prediction}")
                    logger.trace(f"Min price prediction: {min_price_prediction}")
                    logger.trace("-" * 40)
                    
                    realized_profit = self.playground.get_realized_profit()
                    print(f"Realized profit: {realized_profit}")
                    
                    symbol = self.playground.symbol
                    open_trade_count = len(self.playground.fetch_open_orders(symbol))
                    print(f"Open trade count: {open_trade_count}")
                    
                    additional_equity_risk = 0
                    if realized_profit > 0 and open_trade_count < 2:
                        additional_equity_risk = realized_profit * self.additional_profit_risk_percentage
                    
                    open_signals.append(
                        OpenSignalV2(
                            open_signal, 
                            date, 
                            max_price_prediction, 
                            min_price_prediction,
                            additional_equity_risk
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
                
            target_set = add_supertrend_momentum_signal_target_set_v2(self.feature_set, self.min_max_window_in_hours)
            self.factory = new_supertrend_momentum_signal_factory(target_set)
                    
        return open_signals

def add_supertrend_momentum_signal_target_set_v2(df: pd.DataFrame, min_max_window_in_hours: float) -> pd.DataFrame:
    # Initialize new columns
    df['min_price_prediction'] = None
    df['max_price_prediction'] = None
    df['last_close_price'] = None
    df['min_price_prediction_time'] = None
    df['max_price_prediction_time'] = None
    df['last_close_price_time'] = None

    # Calculate min, max, and close prices within min_max_period_in_hours for rows where cross_below_80 is True
    for idx, row in df[df['stochrsi_cross_below_80']].iterrows():
        start_time = row['date']
        end_time = start_time + pd.Timedelta(hours=min_max_window_in_hours)
        mask = (df['date'] > start_time) & (df['date'] <= end_time)
        
        if not df.loc[mask].empty:
            min_price_idx = df.loc[mask, 'low'].idxmin()
            max_price_idx = df.loc[mask, 'high'].idxmax()
            close_price_row = df.loc[mask].iloc[-1] if not df.loc[mask].empty else None
            
            df.loc[idx, 'min_price_prediction'] = df.loc[min_price_idx, 'low']
            df.loc[idx, 'max_price_prediction'] = df.loc[max_price_idx, 'high']
            df.loc[idx, 'last_close_price'] = close_price_row['close'] if close_price_row is not None else None
            
            df.loc[idx, 'min_price_prediction_time'] = df.loc[min_price_idx, 'date']
            df.loc[idx, 'max_price_prediction_time'] = df.loc[max_price_idx, 'date']
            df.loc[idx, 'last_close_price_time'] = close_price_row['date'] if close_price_row is not None else None

    # Calculate min, max, and close prices within 1 day for rows where cross_above_20 is True
    for idx, row in df[df['stochrsi_cross_above_20']].iterrows():
        start_time = row['date']
        end_time = start_time + pd.Timedelta(hours=min_max_window_in_hours)
        mask = (df['date'] > start_time) & (df['date'] <= end_time)
        
        if not df.loc[mask].empty:
            min_price_idx = df.loc[mask, 'low'].idxmin()
            max_price_idx = df.loc[mask, 'high'].idxmax()
            close_price_row = df.loc[mask].iloc[-1] if not df.loc[mask].empty else None
            
            df.loc[idx, 'min_price_prediction'] = df.loc[min_price_idx, 'low']
            df.loc[idx, 'max_price_prediction'] = df.loc[max_price_idx, 'high']
            df.loc[idx, 'last_close_price'] = close_price_row['close'] if close_price_row is not None else None
            
            df.loc[idx, 'min_price_prediction_time'] = df.loc[min_price_idx, 'date']
            df.loc[idx, 'max_price_prediction_time'] = df.loc[max_price_idx, 'date']
            df.loc[idx, 'last_close_price_time'] = close_price_row['date'] if close_price_row is not None else None

    # Filter the DataFrame to include only rows with cross_below_80 or cross_above_20
    filtered_df = df[(df['stochrsi_cross_below_80']) | (df['stochrsi_cross_above_20'])]

    # Handle NaN values by filling them with 0
    filtered_df = filtered_df.fillna(0)
    
    return filtered_df


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
    
    strategy = SimpleOpenStrategyV4(playground, updateFrequency)
    
    while not strategy.is_complete():
        strategy.tick()
        
    logger.info("Done")