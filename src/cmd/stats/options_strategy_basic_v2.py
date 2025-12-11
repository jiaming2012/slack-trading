from datetime import datetime, timedelta
from typing import List, Tuple
from loguru import logger
from dateutil.parser import isoparse

import pandas as pd
from backtester_playground_client_grpc import BacktesterPlaygroundClient, Repository, RepositorySource, CreatePolygonPlaygroundRequest, PlaygroundEnvironment, OrderSide
from rpc.playground_pb2 import GetOptionsLadderRequest, OptionLadderContract
from base_open_strategy_v2 import BaseOpenStrategyV2
from trading_engine_types import OpenSignalV3, OpenSignalName
from rpc.playground_pb2 import Candle

# v2 closes an assigned options by buying the underlying stock at market price

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

    def check_for_new_signal(self) -> OpenSignalName:
        latest_candle = self.candles_ltf.iloc[self.candles_ltf_idx - 1]
        signal = None
        # Only check for new signals on Mondays
        dt = isoparse(latest_candle['datetime'])
        if dt.weekday() == 0:
            signal = OpenSignalName.LONG_OPTION_ENTRY
        
        return signal

    def find_next_friday(self, current_date: datetime) -> int:
        days_ahead = 4 - current_date.weekday()  # Friday is 4
        if days_ahead <= 0:
            days_ahead += 7
        return days_ahead
    
    def find_target_option_contract(self, current_price, contracts) -> OptionLadderContract:
        closest_contract = None
        smallest_diff = float('inf')
        
        for contract in contracts:
            if contract.type != 'call':
                continue
            
            strike_price = contract.strike
            diff = abs(strike_price - current_price)
            if diff < smallest_diff:
                smallest_diff = diff
                closest_contract = contract
                
        return closest_contract
                               
    def tick(self, tick_delta) -> List[OpenSignalV3]:        
        # Check for assigned options
        position = self.playground.account.positions.get(self.symbol, None)
        if position is not None:
            qty = position.quantity
            if qty < 0:
                requested_price = playground.get_current_candle(self.symbol, playground.ltf_seconds).close
                
                # Buy the underlying stock at market price to close the assigned options
                self.playground.place_order(
                    self.symbol,
                    abs(qty),
                    OrderSide.BUY_TO_COVER,
                    'equity',
                    requested_price
                )
                
                self.logger.info(f"Closed assigned options by buying {abs(qty)} shares of {self.symbol} at market price.")

        # Check for new open signals
        open_signals = []
        new_candles: List[Candle] = tick_delta.new_candles if hasattr(tick_delta, 'new_candles') else []
        for c in new_candles:
            self.logger.trace(f"new candle - {c.period} @ {c.bar.datetime} - {c.bar.close}")
            
            if not c.period == self.playground.ltf_seconds:
                continue
            
            self.logger.trace(f"Processing LTF candle - {c.period} @ {c.bar.datetime} - {c.bar.close}")
                
            self.append_candle(c.bar)
            
            signal_name = self.check_for_new_signal()

            if signal_name == OpenSignalName.LONG_OPTION_ENTRY:
                print(f"New signal detected: {signal_name}")
                
                open_signal = OpenSignalV3(
                    name=signal_name,
                    symbol=self.symbol,
                    timestamp=self.playground.timestamp,
                    kwargs={
                        "current_price": c.bar.close,
                    },
                    additional_equity_risk=0.0
                )
                
                open_signals.append(open_signal)

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
            open_signals = strategy.tick(tick_delta)
            
            for signal in open_signals:
                expiration_in_days = strategy.find_next_friday(signal.timestamp)
                
                request = GetOptionsLadderRequest(
                    playground_id=playground.id,
                    stock_symbol=signal.symbol,
                    max_no_of_strikes=5,
                    min_distance_between_strikes=1.0,
                    expiration_in_days=[expiration_in_days],
                    max_tick_age_in_minutes=1440
                )
                
                response = playground.fetch_ladder(request)
                if response:
                    target_contract = strategy.find_target_option_contract(
                        current_price=signal.kwargs['current_price'],
                        contracts=response.contracts
                    )
                    
                    if target_contract is None:
                        logger.warning(f"No suitable option contract found for {signal.symbol} at price {signal.kwargs['current_price']}")
                        continue
                    
                    playground.place_order(
                        target_contract.symbol, 
                        1, 
                        OrderSide.SELL_TO_OPEN,
                        'option'
                    )
                    
                    logger.info(f"Open Signal: {signal.name} at {signal.timestamp} for {signal.symbol}")
                
        playground.tick(playground.ltf_seconds)
        

    logger.info("Done")