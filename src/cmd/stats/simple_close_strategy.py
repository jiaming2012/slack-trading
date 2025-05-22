from backtester_playground_client_grpc import OrderSide
from dataclasses import dataclass
from rpc.playground_pb2 import Order
from typing import List
from datetime import datetime
from dateutil.parser import isoparse
import pandas as pd
import re

@dataclass
class CloseSignal:
    Timestamp: pd.Timestamp
    OrderId: str
    Symbol: str
    Side: OrderSide
    Volume: float
    Reason: str

def parse_order_tag(tag):
    # Define the regular expression pattern
    pattern = r"sl--(?P<sl>\d+-\d+)--tp--(?P<tp>\d+-\d+)"
    
    # Match the pattern with the tag
    match = re.match(pattern, tag)
    
    if match:
        # Extract the sl and tp values
        sl = match.group('sl').replace('-', '.')
        tp = match.group('tp').replace('-', '.')
        return float(sl), float(tp)
    else:
        raise ValueError("Invalid tag format")

def calc_open_quantity(order: Order) -> float:
    qty = 0
    for t in order.trades:
        qty += t.quantity
    return qty

def calc_remaining_open_quantity(order: Order) -> float:
    qty = calc_open_quantity(order)
    for t in order.closed_by:
        qty += t.quantity
    return qty

class SimpleCloseStrategy():
    def __init__(self, playground, kwargs: dict):        
        self.symbols = playground.account.meta.symbols
        self.playground = playground
        
    def tick(self, current_price: float, kwargs: dict) -> List[CloseSignal]:
        signals = []
        if not current_price:
            return signals
        
        period = kwargs.get('period')
        if period is None:
            raise ValueError("Period is not set")
        
        for symbol in self.symbols:
            open_orders: List[Order] = self.playground.fetch_open_orders(symbol)
            for open_order in open_orders:
                tag = open_order.tag
                
                try:
                    sl, tp = parse_order_tag(tag)
                except ValueError:
                    continue
                
                current_candle = self.playground.get_current_candle(symbol, period)
                # ts = isoparse(current_candle.datetime)
                ts = datetime.now()
                if open_order.side == OrderSide.BUY.value:
                    if current_price <= sl:
                        qty = calc_remaining_open_quantity(open_order)
                        signals.append(CloseSignal(ts, open_order.id, symbol, OrderSide.SELL, abs(qty), 'sl'))
                    elif current_price >= tp:
                        qty = calc_remaining_open_quantity(open_order)
                        signals.append(CloseSignal(ts, open_order.id, symbol, OrderSide.SELL, abs(qty), 'tp'))
                elif open_order.side == OrderSide.SELL_SHORT.value:
                    if current_price >= sl:
                        qty = calc_remaining_open_quantity(open_order)
                        signals.append(CloseSignal(ts, open_order.id, symbol, OrderSide.BUY_TO_COVER, abs(qty), 'sl'))
                    elif current_price <= tp:
                        qty = calc_remaining_open_quantity(open_order)
                        signals.append(CloseSignal(ts, open_order.id, symbol, OrderSide.BUY_TO_COVER, abs(qty), 'tp'))
                else:
                    raise ValueError("Invalid side")
                
        return signals

if __name__ == "__main__":
    from backtester_playground_client_grpc import BacktesterPlaygroundClient, Repository, RepositorySource, PlaygroundEnvironment, CreatePolygonPlaygroundRequest
    from loguru import logger

    symbol = "AAPL"
    balance = 10000
    start_date = None
    stop_date = None
    ltf_repo = Repository(
        symbol=symbol,
        timespan_multiplier=5,
        timespan_unit='minute',
        indicators=["supertrend"],
        history_in_days=10
    )
    env = PlaygroundEnvironment.LIVE
    
    req = CreatePolygonPlaygroundRequest(
        balance=balance,
        start_date=start_date,
        stop_date=stop_date,
        repositories=[ltf_repo],
        environment=env.value,
        client_id="simple_close_strategy_test2abc",
        tags=[symbol.lower(), "simple_close_strategy_test2abc"],
    )
    
    live_account_type = "margin"
    # twirp_host = "http://45.77.223.21"
    twirp_host = "http://localhost:5051"
    repository_source = RepositorySource.POLYGON
    
    playground = BacktesterPlaygroundClient(req, live_account_type, repository_source, logger, twirp_host=twirp_host)
    
    # Example usage
    strategy = SimpleCloseStrategy(playground=playground, kwargs={})
    signals = strategy.tick(current_price=150.0, kwargs={
        'period': 300
    })
    for signal in signals:
        print(signal)