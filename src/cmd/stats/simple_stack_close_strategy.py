from backtester_playground_client_grpc import OrderSide
from dataclasses import dataclass
from typing import Tuple
from rpc.playground_pb2 import Order
from typing import List
from datetime import datetime
from zoneinfo import ZoneInfo
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

def avg_filled_price(order: Order) -> float:
    if len(order.trades) == 0:
        raise ValueError("Order has no trades")
    
    total_qty = abs(sum(t.quantity for t in order.trades))
    vwap = 0.0
    for t in order.trades:
        vwap += t.price * (abs(t.quantity) / total_qty)
    return vwap

def total_risk_taken(orders: List[Order]) -> Tuple[float, float]:        
    total_risk = 0
    total_quantity = 0
    order_fills: Tuple[float, float] = []
    for order in orders:
        entry_price = avg_filled_price(order)
        total_risk += abs(entry_price - order.stop_price) * abs(order.quantity)
        total_quantity += order.quantity
        order_fills.append((entry_price, order.quantity))
    
    if total_quantity == 0:
        raise ValueError("No open orders")
        
    avg_entry_price = 0.0
    for price, qty in order_fills:
        avg_entry_price += price * (abs(qty) / total_quantity)
        
    return total_risk, avg_entry_price

def target_tp_for_reward_ratio(orders: List[Order], reward_risk_ratio: float) -> float:
    total_quantity = sum(order.quantity for order in orders)
    total_risk, avg_entry_price = total_risk_taken(orders)
    total_reward = reward_risk_ratio * total_risk

    # TP that would yield the desired reward
    tp = total_reward / total_quantity + avg_entry_price
    return tp

class SimpleStackCloseStrategy():
    def __init__(self, playground, logger, max_open_count, target_risk_to_reward: float):        
        self.symbols = playground.account.meta.symbols
        self.playground = playground
        self.logger = logger
        self.max_open_count = max_open_count
        self.target_risk_to_reward = target_risk_to_reward
        
    def tick(self, symbol: str, current_price: float, kwargs: dict) -> List[CloseSignal]:
        signals = []
        if not current_price:
            return signals
        
        playground = kwargs.get('playground')
        if playground is None:
            raise ValueError("Playground is not set")
        
        period = kwargs.get('period')
        if period is None:
            raise ValueError("Period is not set")
        
        supertrend_direction = kwargs['supertrend_direction']

        current_candle = playground.get_current_candle(symbol, period)
        
        candle_dt = isoparse(current_candle.datetime)
        
        if self.playground.environment == 'simulator':
            ts = candle_dt.astimezone(ZoneInfo("America/New_York"))
        else:
            ts = datetime.now(tz=candle_dt.tzinfo)
            
        open_orders: List[Order] = self.playground.fetch_open_orders(symbol)   
        for open_order in open_orders:
            tag = open_order.tag
            
            # Check SL
            if open_order.side == OrderSide.BUY.value:
                if current_price <= open_order.stop_price:
                    qty = calc_remaining_open_quantity(open_order)
                    signals.append(CloseSignal(ts, open_order.id, symbol, OrderSide.SELL, abs(qty), 'sl'))
                    continue
            elif open_order.side == OrderSide.SELL_SHORT.value:
                if current_price >= open_order.stop_price:
                    qty = calc_remaining_open_quantity(open_order)
                    signals.append(CloseSignal(ts, open_order.id, symbol, OrderSide.BUY_TO_COVER, abs(qty), 'sl'))
                    continue
            else:
                raise ValueError("Check SL: Invalid side")
            
            # Check TP
            try:
                count = parse_order_tag(tag)
            except ValueError:
                self.logger.error(f"Invalid tag format: {tag}")
                continue
            
            if count >= self.max_open_count:
                tp = target_tp_for_reward_ratio(open_orders, self.target_risk_to_reward)
                if open_order.side == OrderSide.BUY.value:
                    tp += kwargs['tp_buffer']
                    if current_price >= tp:
                        total_qty = sum(calc_remaining_open_quantity(order) for order in open_orders)
                        signals.append(CloseSignal(ts, None, symbol, OrderSide.SELL, abs(total_qty), 'tp'))
                        continue
                elif open_order.side == OrderSide.SELL_SHORT.value:
                    tp -= kwargs['tp_buffer']
                    if current_price <= tp:
                        total_qty = sum(calc_remaining_open_quantity(order) for order in open_orders)
                        signals.append(CloseSignal(ts, None, symbol, OrderSide.BUY_TO_COVER, abs(total_qty), 'tp'))
                        continue
                else:
                    raise ValueError("Check TP: Invalid side")
                
            # Check if the supertrend direction has changed
            if open_order.side == OrderSide.BUY.value and supertrend_direction == -1:
                qty = calc_remaining_open_quantity(open_order)
                signals.append(CloseSignal(ts, open_order.id, symbol, OrderSide.SELL, abs(qty), 'supertrend'))
                continue
            
            if open_order.side == OrderSide.SELL_SHORT.value and supertrend_direction == 1:
                qty = calc_remaining_open_quantity(open_order)
                signals.append(CloseSignal(ts, open_order.id, symbol, OrderSide.BUY_TO_COVER, abs(qty), 'supertrend'))
                continue
                    
                
        return signals
