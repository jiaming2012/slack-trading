import argparse
from rpc.playground_twirp import PlaygroundServiceClient
from rpc.playground_pb2 import GetAccountRequest, Order, Trade, AccountMeta, Bar
from twirp.context import Context
from pprint import pprint
from typing import List, Dict
from datetime import datetime
from dataclasses import dataclass
from dateutil.parser import parse, ParserError
import re

@dataclass
class TradePosition:
    vwap: float
    quantity: float

def _calc_trade_position(trades: List[Trade]) -> TradePosition:
    total_quantity = sum([trade.quantity for trade in trades])
    vwap = sum([trade.price * trade.quantity for trade in trades]) / total_quantity if total_quantity > 0 else 0

    return TradePosition(vwap=vwap, quantity=total_quantity)

def _parse_timestamp(timestamp: str) -> datetime:
    try:
        parsed_date = parse(timestamp)
    except ParserError:
        timestamp = re.sub(r' [A-Z]{3}$', '', timestamp)
        parsed_date = parse(timestamp)
        
    return parsed_date

def _calc_trade_duration_list_in_seconds(orders) -> List[int]:
    trade_durations = []
    
    for order in orders:
        if order.side == 'buy' or order.side == 'sell_short':
            open_timestamp = _parse_timestamp(order.create_date)
            
            close_timestamps = []
            total_quantity = 0
            for trade in order.closed_by:
                close_timestamps.append((_parse_timestamp(trade.create_date), trade.quantity))
                total_quantity += trade.quantity
                
            weighted_duration_in_seconds = []
            for close_timestamp, quantity in close_timestamps:
                duration = close_timestamp - open_timestamp
                weighted_duration_in_seconds.append(duration.total_seconds() * (quantity / total_quantity))
                
            if len(weighted_duration_in_seconds) > 0:
                trade_durations.append(sum(weighted_duration_in_seconds))

        else:
            continue
        
    return trade_durations

def _calc_realized_profit_list(orders) -> List[float]:    
    realized_profits = []
    
    for order in orders:
        pl = 0
        
        if order.side == 'buy':
            trade_position = _calc_trade_position(order.trades)
            if trade_position.vwap > 0:
                for trade in order.closed_by:
                    pl += (trade.price - trade_position.vwap) * abs(trade.quantity)
                
        elif order.side == 'sell_short':
            trade_position = _calc_trade_position(order.trades)
            if trade_position.vwap > 0:
                for trade in order.closed_by:
                    pl += (trade_position.vwap - trade.price) * trade.quantity
                
        else:
            continue
        
        realized_profits.append(pl)
        
    return realized_profits

def calc_positions(orders) -> Dict[str, TradePosition]:
    positions = {}

    for order in orders:
        pos = positions.get(order.symbol, TradePosition(vwap=0, quantity=0))
        trades = _calc_trade_position(order.trades)
        
        if pos.quantity > 0:
            if trades.quantity > 0:
                pos.vwap = (pos.vwap * pos.quantity + trades.vwap * trades.quantity) / (pos.quantity + trades.quantity)
            
            pos.quantity += trades.quantity
        elif pos.quantity < 0:
            if trades.quantity < 0:
                pos.vwap = (pos.vwap * pos.quantity + trades.vwap * trades.quantity) / (pos.quantity + trades.quantity)
            
            pos.quantity += trades.quantity
        else:
            pos.vwap = trades.vwap
            pos.quantity = trades.quantity
            
        if pos.quantity == 0:
            pos.vwap = 0
            
        positions[order.symbol] = pos

    return positions

def calc_gross_profit(profits : List[float]) -> float:
    return sum([profit for profit in profits if profit > 0])

def calc_gross_loss(profits : List[float]) -> float:
    return sum([profit for profit in profits if profit < 0])

def calc_total_orders(orders) -> int:
    return len(orders)

def calc_slippage(orders) -> float:
    slippage_in_points = []
    slippage_in_dollars = []
    max_slippage_order_id = None
    min_slippage_order_id = None
    max_slippage = None
    min_slippage = None
    
    for order in orders:
        val = None
        
        if order.side == 'buy':
            val = order.trades[0].price - order.requested_price
            slippage_in_points.append(val)
            
            valInDollars = val * order.trades[0].quantity
            slippage_in_dollars.append(valInDollars)
        elif order.side == 'sell_short':
            val = order.requested_price - order.trades[0].price
            slippage_in_points.append(val)
            
            valInDollars = val * order.trades[0].quantity
            slippage_in_dollars.append(valInDollars)
            
        if val:
            if max_slippage is None or valInDollars > max_slippage:
                max_slippage = valInDollars
                max_slippage_order_id = order.id
                
            if min_slippage is None or valInDollars < min_slippage:
                min_slippage = valInDollars
                min_slippage_order_id = order.id
                
        
    results = {}
    results['total_slippage_in_points'] = sum(slippage_in_points)
    results['total_slippage_in_dollars'] = sum(slippage_in_dollars)
    results['avg_slippage_in_dollars'] = sum(slippage_in_dollars) / len(slippage_in_dollars) if len(slippage_in_dollars) > 0 else 'n/a'
    results['max_slippage_in_dollars'] = max_slippage
    results['max_slippage_order_id'] = max_slippage_order_id
    results['min_slippage_in_dollars'] = min_slippage
    results['min_slippage_order_id'] = min_slippage_order_id
    
    standard_deviation = (sum([(s - results['avg_slippage_in_dollars']) ** 2 for s in slippage_in_dollars]) / len(slippage_in_dollars)) ** 0.5
    results['std_dev_slippage_in_dollars'] = standard_deviation if len(slippage_in_dollars) > 0 else 'n/a'

    return results

def calc_total_trades(orders) -> int:
    trade_count = 0

    for order in orders:
        if order.side == 'buy':
            trade_count += 1
        elif order.side == 'sell_short':
            trade_count += 1

    return trade_count

def calc_realized_profit(profits: List[float]) -> float:
    return sum(profits)

def calc_avg_profit(profits: List[float]) -> float:
    profs = [pl for pl in profits if pl > 0]
    return sum(profs) / len(profs) if len(profs) > 0 else 'n/a'

def calc_avg_loss(profits: List[float]) -> float:
    losses = [pl for pl in profits if pl < 0]
    return sum(losses) / len(losses) if len(losses) > 0 else 'n/a'

def calc_winners_count(profits: List[float]) -> int:
    return len([profit for profit in profits if profit > 0])

def calc_losers_count(profits: List[float]) -> int:
    return len([profit for profit in profits if profit < 0])

def calc_breakeven_count(profits: List[float]) -> int:
    return len([profit for profit in profits if profit == 0])

def collect_data(host: str, playground_id: str, from_date: str = None, to_date: str = None) -> dict:
    client = PlaygroundServiceClient(host, timeout=60)

    req = GetAccountRequest(
            playground_id=playground_id, 
            fetch_orders=True,
            status=['filled'],   
        )
    
    if from_date:
        req.fromRTF339 = f'{from_date}T00:00:00Z'
        
    if to_date:
        req.toRTF339 = f'{to_date}T00:00:00Z'
    
    acc = client.GetAccount(
        ctx=Context(),
        request=req
    )
    
    # orders = acc.orders
    orders = [o for o in acc.orders if o.status == 'filled']

    profit_list = _calc_realized_profit_list(orders)
    trade_duration_list_in_seconds = _calc_trade_duration_list_in_seconds(orders)

    gross_data = {}
    gross_data['total_orders'] = calc_total_orders(orders)
    gross_data['total_trades'] = calc_total_trades(orders)
    gross_data['gross_profit'] = calc_gross_profit(profit_list)
    gross_data['gross_loss'] = calc_gross_loss(profit_list)
    gross_data['winners_count'] = calc_winners_count(profit_list)
    gross_data['losers_count'] = calc_losers_count(profit_list)
    gross_data['breakeven_count'] = calc_breakeven_count(profit_list)
    gross_data['avg_profit'] = calc_avg_profit(profit_list)
    gross_data['avg_loss'] = calc_avg_loss(profit_list)
    gross_data['min_trade_duration_in_minutes'] = min(trade_duration_list_in_seconds) / 60.0 if len(trade_duration_list_in_seconds) > 0 else 'n/a'
    gross_data['max_trade_duration_in_minutes'] = max(trade_duration_list_in_seconds) / 60.0 if len(trade_duration_list_in_seconds) > 0 else 'n/a'
    gross_data['positions'] = calc_positions(orders)
    gross_data['slippage'] = calc_slippage(orders)

    agg_data = {}
    agg_data['profit_factor'] = gross_data['gross_profit'] / abs(gross_data['gross_loss']) if gross_data['gross_loss'] != 0 else 'n/a'
    agg_data['realized_profit'] = calc_realized_profit(profit_list)
    agg_data['win_rate'] = gross_data['winners_count'] / gross_data['total_trades'] if gross_data['total_trades'] != 0 else 'n/a'
    agg_data['avg_trade_duration_in_minutes'] = sum(trade_duration_list_in_seconds) / len(trade_duration_list_in_seconds) / 60.0 if len(trade_duration_list_in_seconds) > 0 else 'n/a'

    return {'gross_data': gross_data, 'agg_data': agg_data}

if __name__ == '__main__':
    args = argparse.ArgumentParser()
    args.add_argument('--playground-id', type=str, required=True, help="Playground ID")
    args.add_argument('--twirp-host', type=str, default='http://localhost:5051', help="twirp rpc host")
    args.add_argument('--from-date', type=str, default=None, help="start date")
    args.add_argument('--to-date', type=str, default=None, help="end date")

    args = args.parse_args()

    data = collect_data(args.twirp_host, args.playground_id, args.from_date, args.to_date)

    print('gross data:')
    pprint(data['gross_data'])

    print('agg data:')
    pprint(data['agg_data'])
