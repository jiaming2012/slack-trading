from backtester_playground_client_grpc import OrderSide
from dataclasses import dataclass
from rpc.playground_pb2 import Order
from typing import List
import re

MaxOpenOrders = 3
TargetRiskToReward = 2.0

@dataclass
class CloseSignal:
    OrderId: str
    Symbol: str
    Side: OrderSide
    Volume: float
    Reason: str

def parse_order_tag(tag) -> int:
    # Define the regular expression pattern
    pattern = r"SupertrendStackSignal-(?P<count>\d+)"
    
    # Match the pattern with the tag
    match = re.match(pattern, tag)
    
    if match:
        count = match.group('count')
        return int(count)
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

def total_risk_taken(orders: List[Order]) -> float:        
    total_risk = 0
    for order in orders:
        total_risk += abs(order.price - order.stop_price) * order.quantity
        
    return total_risk

def target_tp_for_reward_ratio(orders: List[Order], reward_risk_ratio: float) -> float:
    total_quantity = sum(order.quantity for order in orders)
    total_risk = total_risk_taken(orders)
    total_reward = reward_risk_ratio * total_risk

    # Weighted average entry price
    avg_entry_price = sum(order.price * order.quantity for order in orders) / total_quantity

    # TP that would yield the desired reward
    tp = total_reward / total_quantity + avg_entry_price
    return tp

class SimpleStackCloseStrategy():
    def __init__(self, playground, logger):        
        self.symbols = playground.account.meta.symbols
        self.playground = playground
        self.logger = logger
        
    def tick(self, current_price: float, kwargs: dict) -> List[CloseSignal]:
        signals = []
        if not current_price:
            return signals
        
        supertrend_direction = kwargs['supertrend_direction']
        for symbol in self.symbols:
            open_orders: List[Order] = self.playground.fetch_open_orders(symbol)   
            for open_order in open_orders:
                tag = open_order.tag
                
                # Check SL
                if open_order.side == OrderSide.BUY.value:
                    if current_price <= open_order.stop_price:
                        qty = calc_remaining_open_quantity(open_order)
                        signals.append(CloseSignal(open_order.id, symbol, OrderSide.SELL, abs(qty), 'sl'))
                        continue
                elif open_order.side == OrderSide.SELL_SHORT.value:
                    if current_price >= open_order.stop_price:
                        qty = calc_remaining_open_quantity(open_order)
                        signals.append(CloseSignal(open_order.id, symbol, OrderSide.BUY_TO_COVER, abs(qty), 'sl'))
                        continue
                else:
                    raise ValueError("Check SL: Invalid side")
                
                # Check TP
                try:
                    count = parse_order_tag(tag)
                except ValueError:
                    self.logger.error(f"Invalid tag format: {tag}")
                    continue
                
                if count >= MaxOpenOrders:
                    tp = target_tp_for_reward_ratio(open_orders, TargetRiskToReward)
                    if open_order.side == OrderSide.BUY.value:
                        if current_price >= tp:
                            total_qty = sum(calc_remaining_open_quantity(order) for order in open_orders)
                            signals.append(CloseSignal(None, symbol, OrderSide.SELL, abs(total_qty), 'tp'))
                            continue
                    elif open_order.side == OrderSide.SELL_SHORT.value:
                        if current_price <= tp:
                            total_qty = sum(calc_remaining_open_quantity(order) for order in open_orders)
                            signals.append(CloseSignal(None, symbol, OrderSide.BUY_TO_COVER, abs(total_qty), 'tp'))
                            continue
                    else:
                        raise ValueError("Check TP: Invalid side")
                    
                # Check if the supertrend direction has changed
                if open_order.side == OrderSide.BUY.value and supertrend_direction == -1:
                    qty = calc_remaining_open_quantity(open_order)
                    signals.append(CloseSignal(open_order.id, symbol, OrderSide.SELL, abs(qty), 'supertrend'))
                    continue
                
                if open_order.side == OrderSide.SELL_SHORT.value and supertrend_direction == 1:
                    qty = calc_remaining_open_quantity(open_order)
                    signals.append(CloseSignal(open_order.id, symbol, OrderSide.BUY_TO_COVER, abs(qty), 'supertrend'))
                    continue
                    
                
        return signals
