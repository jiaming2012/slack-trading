from simple_base_strategy import SimpleBaseStrategy
from backtester_playground_client_grpc import OrderSide
from dataclasses import dataclass
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
    
class SimpleCloseStrategy(SimpleBaseStrategy):
    def __init__(self, playground):
        super().__init__(playground)
        
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
                        signals.append(CloseSignal(symbol, OrderSide.SELL, open_order.quantity, 'sl'))
                    elif current_price >= tp:
                        signals.append(CloseSignal(symbol, OrderSide.SELL, open_order.quantity, 'tp'))
                elif open_order.side == OrderSide.SELL_SHORT.value:
                    if current_price >= sl:
                        signals.append(CloseSignal(symbol, OrderSide.BUY_TO_COVER, open_order.quantity, 'sl'))
                    elif current_price <= tp:
                        signals.append(CloseSignal(symbol, OrderSide.BUY_TO_COVER, open_order.quantity, 'tp'))
                else:
                    raise ValueError("Invalid side")
                
        return signals
