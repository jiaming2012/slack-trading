import argparse
from rpc.playground_twirp import PlaygroundServiceClient
from rpc.playground_pb2 import GetAccountRequest, Order, Trade, AccountMeta, Bar
from twirp.context import Context
from pprint import pprint
from typing import List
from datetime import datetime
from dataclasses import dataclass

@dataclass
class TradePosition:
    vwap: float
    quantity: float

def _calc_trade_position(trades: List[Trade]) -> TradePosition:
    total_quantity = sum([trade.quantity for trade in trades])
    vwap = sum([trade.price * trade.quantity for trade in trades]) / total_quantity

    return TradePosition(vwap=vwap, quantity=total_quantity)

def _parse_timestamp(timestamp: str) -> datetime:
    date_format = "%Y-%m-%d %H:%M:%S %z %Z"
    parsed_date = datetime.strptime(timestamp, date_format)
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
            for trade in order.closed_by:
                pl += (trade.price - trade_position.vwap) * abs(trade.quantity)
                
        elif order.side == 'sell_short':
            trade_position = _calc_trade_position(order.trades)
            for trade in order.closed_by:
                pl += (trade_position.vwap - trade.price) * trade.quantity
                
        else:
            continue
        
        realized_profits.append(pl)
        
    return realized_profits

def calc_gross_profit(profits : List[float]) -> float:
    return sum([profit for profit in profits if profit > 0])

def calc_gross_loss(profits : List[float]) -> float:
    return sum([profit for profit in profits if profit < 0])

def calc_total_orders(orders) -> int:
    return len(orders)

def calc_total_trades(orders) -> int:
    trade_count = 0

    for order in orders:
        if order.side == 'buy':
            trade_count += 1
        elif order.side == 'sell_short':
            trade_count += 1

    return trade_count

def calc_net_profit(profits: List[float]) -> float:
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

args = argparse.ArgumentParser()
args.add_argument('--playground-id', type=str, required=True, help="Playground ID")
args.add_argument('--twirp-host', type=str, default='http://localhost:5051', help="twirp rpc host")

args = args.parse_args()

print(f'fetching metrics from playground id: {args.playground_id}')

client = PlaygroundServiceClient(args.twirp_host, timeout=60)

acc = client.GetAccount(
    ctx=Context(),
    request=GetAccountRequest(playground_id=args.playground_id, fetch_orders=True)
)

profit_list = _calc_realized_profit_list(acc.orders)
trade_duration_list_in_seconds = _calc_trade_duration_list_in_seconds(acc.orders)

gross_data = {}
# gross_data['total_orders'] = calc_total_orders(acc.orders)
gross_data['total_trades'] = calc_total_trades(acc.orders)
gross_data['gross_profit'] = calc_gross_profit(profit_list)
gross_data['gross_loss'] = calc_gross_loss(profit_list)
gross_data['winners_count'] = calc_winners_count(profit_list)
gross_data['losers_count'] = calc_losers_count(profit_list)
gross_data['breakeven_count'] = calc_breakeven_count(profit_list)
gross_data['avg_profit'] = calc_avg_profit(profit_list)
gross_data['avg_loss'] = calc_avg_loss(profit_list)
gross_data['min_trade_duration_in_minutes'] = min(trade_duration_list_in_seconds) / 60.0 if len(trade_duration_list_in_seconds) > 0 else 'n/a'
gross_data['max_trade_duration_in_minutes'] = max(trade_duration_list_in_seconds) / 60.0 if len(trade_duration_list_in_seconds) > 0 else 'n/a'

agg_data = {}
agg_data['profit_factor'] = gross_data['gross_profit'] / abs(gross_data['gross_loss']) if gross_data['gross_loss'] != 0 else 'n/a'
agg_data['net_profit'] = calc_net_profit(profit_list)
agg_data['win_rate'] = gross_data['winners_count'] / gross_data['total_trades'] if gross_data['total_trades'] != 0 else 'n/a'
agg_data['avg_trade_duration_in_minutes'] = sum(trade_duration_list_in_seconds) / len(trade_duration_list_in_seconds) / 60.0 if len(trade_duration_list_in_seconds) > 0 else 'n/a'

print('gross data:')
pprint(gross_data)

print('agg data:')
pprint(agg_data)

print(f'profit_list: {_calc_realized_profit_list(acc.orders)}')
print(f'trade_duration_list_in_seconds: {_calc_trade_duration_list_in_seconds(acc.orders)}')