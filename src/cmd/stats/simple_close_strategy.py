from backtester_playground_client_grpc import OrderSide
from dataclasses import dataclass
from rpc.playground_pb2 import Order
from typing import List
import re

@dataclass
class CloseSignal:
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
    def __init__(self, playground):        
        self.symbols = playground.account.meta.symbols
        self.playground = playground
        
    def tick(self, current_price: float) -> List[CloseSignal]:
        signals = []
        if not current_price:
            return signals
        
        for symbol in self.symbols:
            open_orders = self.playground.fetch_open_orders(symbol)
            for open_order in open_orders:
                tag = open_order.tag
                
                try:
                    sl, tp = parse_order_tag(tag)
                except ValueError:
                    continue
                
                if open_order.side == OrderSide.BUY.value:
                    if current_price <= sl:
                        qty = calc_remaining_open_quantity(open_order)
                        signals.append(CloseSignal(symbol, OrderSide.SELL, abs(qty), 'sl'))
                    elif current_price >= tp:
                        qty = calc_remaining_open_quantity(open_order)
                        signals.append(CloseSignal(symbol, OrderSide.SELL, abs(qty), 'tp'))
                elif open_order.side == OrderSide.SELL_SHORT.value:
                    if current_price >= sl:
                        qty = calc_remaining_open_quantity(open_order)
                        signals.append(CloseSignal(symbol, OrderSide.BUY_TO_COVER, abs(qty), 'sl'))
                    elif current_price <= tp:
                        qty = calc_remaining_open_quantity(open_order)
                        signals.append(CloseSignal(symbol, OrderSide.BUY_TO_COVER, abs(qty), 'tp'))
                else:
                    raise ValueError("Invalid side")
                
        return signals
