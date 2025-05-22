import argparse
from rpc.playground_twirp import PlaygroundServiceClient
from rpc.playground_pb2 import GetAccountRequest, GetAccountResponse, GetPlaygroundsRequest, GetPriceFromBroker, Order, Trade, AccountMeta, Bar, Position
from twirp.context import Context
from pprint import pprint
from typing import List, Dict, Tuple
from datetime import datetime
from dataclasses import dataclass
from dateutil.parser import parse, ParserError
from pytz import timezone, UTC
import re

@dataclass
class TradePosition:
    vwap: float
    quantity: float
    current_price: float
    pl: float
    
def fetch_playground_ids(client: PlaygroundServiceClient, tags: List[str]) -> List[str]:
    req = GetPlaygroundsRequest(tags=tags)
    
    resp = client.GetPlaygrounds(
        ctx=Context(),
        request=req
    )
    
    return [p.playground_id for p in resp.playgrounds]

def _calc_trade_position(trades: List[Trade]) -> TradePosition:
    total_quantity = sum([trade.quantity for trade in trades])
    vwap = sum([trade.price * trade.quantity for trade in trades]) / total_quantity if total_quantity != 0 else 0

    return TradePosition(vwap=vwap, quantity=total_quantity, current_price=0, pl=0)

def _parse_timestamp(timestamp: str) -> datetime:
    try:
        parsed_date = parse(timestamp)
    except ParserError:
        timestamp = re.sub(r' [A-Z]{3}$', '', timestamp)
        parsed_date = parse(timestamp)
        
     # Convert the parsed datetime to UTC if it has no timezone info
    if parsed_date.tzinfo is None:
        parsed_date = UTC.localize(parsed_date)
    
    # Convert the UTC datetime to EST
    est = timezone('US/Eastern')
    return parsed_date.astimezone(est)

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

def _calc_realized_order_profit(order) -> Tuple[float, float, float]:
    pl = 0
    open_price = 0
    close_prices = []
    
    if order.side == 'buy':
        open_position = _calc_trade_position(order.trades)
        open_price = open_position.vwap
        if open_price > 0:
            for trade in order.closed_by:
                pl += (trade.price - open_price) * abs(trade.quantity)
                close_prices.append(trade.price)
                
    elif order.side == 'buy_to_cover':
        if len(order.closes) == 0:
            raise ValueError('buy_to_cover order has no closes')
        
        total_quantity = 0
        for o in order.closes:
            total_quantity += sum([trade.quantity for trade in o.trades])
        
        open_position = TradePosition(vwap=0, quantity=0, current_price=0, pl=0)
        if total_quantity < 0:
            for trade in order.closes:
                p = _calc_trade_position(trade.trades)
                open_position.vwap += (p.vwap * p.quantity) / total_quantity
                open_position.quantity += p.quantity
                
        open_price = open_position.vwap
        if open_price <= 0:
            raise ValueError('buy_to_cover order has no open price')
        
        for trade in order.trades:
            pl += (open_price - trade.price) * abs(trade.quantity)
            close_prices.append(trade.price)
                
    elif order.side == 'sell':
        if len(order.closes) == 0:
            raise ValueError('sell order has no closes')
        
        total_quantity = 0
        for o in order.closes:
            total_quantity += sum([trade.quantity for trade in o.trades])
        
        open_position = TradePosition(vwap=0, quantity=0, current_price=0, pl=0)
        if total_quantity > 0:
            for o in order.closes:
                p = _calc_trade_position(o.trades)
                open_position.vwap += (p.vwap * p.quantity) / total_quantity
                open_position.quantity += p.quantity
                
        open_price = open_position.vwap
        if open_price <= 0:
            raise ValueError('sell order has no open price')
        
        for trade in order.trades:
            pl += (trade.price - open_price) * abs(trade.quantity)
            close_prices.append(trade.price)
                
    elif order.side == 'sell_short':
        open_position = _calc_trade_position(order.trades)
        open_price = open_position.vwap
        if open_price > 0:
            for trade in order.closed_by:
                pl += (open_price - trade.price) * trade.quantity
                close_prices.append(trade.price)
                
    close_price = sum(close_prices) / len(close_prices) if len(close_prices) > 0 else 0
    return pl, open_price, close_price
                    
def _calc_realized_profit_list(orders) -> List[float]:    
    realized_profits = []
    
    for order in orders:
        pl, _, _ = _calc_realized_order_profit(order)
        if pl is None:
            continue
        
        realized_profits.append(pl)
        
    return realized_profits

def calc_positions(orders) -> Dict[str, TradePosition]:
    positions = {}
    b_start_calculation = False

    for order in orders:
        if order.side == 'buy' or order.side == 'sell_short':
            b_start_calculation = True
        
        if not b_start_calculation:
            continue
        
        pos = positions.get(order.symbol, TradePosition(vwap=0, quantity=0, current_price=0, pl=0))
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

def calc_close_order_slippage(order) -> float:
    if order.side == 'sell':
        slippage_in_points = order.requested_price - order.trades[0].price
    elif order.side == 'buy_to_cover':
        slippage_in_points = order.trades[0].price - order.requested_price
    else:
        return None, None

    slippage_in_dollars = slippage_in_points * abs(order.trades[0].quantity)
    return slippage_in_points, slippage_in_dollars

def calc_open_order_slippage(order) -> float:
    if order.side == 'buy':
        slippage_in_points = order.trades[0].price - order.requested_price
    elif order.side == 'sell_short':
        slippage_in_points = order.requested_price - order.trades[0].price
    else:
        return None, None

    slippage_in_dollars = slippage_in_points * abs(order.trades[0].quantity)
    return slippage_in_points, slippage_in_dollars

def calc_close_slippage(orders) -> dict:
    slippage_in_points_list = []
    slippage_in_dollars_list = []
    max_slippage = None
    min_slippage = None
    
    for order in orders:
        val = None
        
        slippage_in_points, slippage_in_dollars = calc_close_order_slippage(order)
        if slippage_in_points:
            if max_slippage is None or slippage_in_dollars > max_slippage:
                max_slippage = slippage_in_dollars
                
            if min_slippage is None or slippage_in_dollars < min_slippage:
                min_slippage = slippage_in_dollars
        
            slippage_in_points_list.append(slippage_in_points)
            slippage_in_dollars_list.append(slippage_in_dollars)
        
    results = {}
    results['total_slippage_in_points'] = sum(slippage_in_points_list)
    results['total_slippage_in_dollars'] = sum(slippage_in_dollars_list)
    results['avg_slippage_in_dollars'] = sum(slippage_in_dollars_list) / len(slippage_in_dollars_list) if len(slippage_in_dollars_list) > 0 else 'n/a'
    results['max_slippage_in_dollars'] = max_slippage
    results['min_slippage_in_dollars'] = min_slippage

    return results

def calc_total_slippage(open_slippage: dict, close_slippage: dict) -> float:
    total_slippage = {}
    total_slippage['total_slippage_in_points'] = open_slippage['total_slippage_in_points'] + close_slippage['total_slippage_in_points']
    total_slippage['total_slippage_in_dollars'] = open_slippage['total_slippage_in_dollars'] + close_slippage['total_slippage_in_dollars']
    
    return total_slippage

def calc_open_slippage(orders) -> dict:
    slippage_in_points_list = []
    slippage_in_dollars_list = []
    max_slippage = None
    min_slippage = None
    
    for order in orders:
        val = None
        
        slippage_in_points, slippage_in_dollars = calc_open_order_slippage(order)
        if slippage_in_points:
            if max_slippage is None or slippage_in_dollars > max_slippage:
                max_slippage = slippage_in_dollars
                
            if min_slippage is None or slippage_in_dollars < min_slippage:
                min_slippage = slippage_in_dollars
        
            slippage_in_points_list.append(slippage_in_points)
            slippage_in_dollars_list.append(slippage_in_dollars)
        
    results = {}
    results['total_slippage_in_points'] = sum(slippage_in_points_list)
    results['total_slippage_in_dollars'] = sum(slippage_in_dollars_list)
    results['avg_slippage_in_dollars'] = sum(slippage_in_dollars_list) / len(slippage_in_dollars_list) if len(slippage_in_dollars_list) > 0 else 'n/a'
    results['max_slippage_in_dollars'] = max_slippage
    results['min_slippage_in_dollars'] = min_slippage

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

def fetch_account(client: PlaygroundServiceClient, playground_id: str, orders_from_date: str = None, orders_to_date: str = None) -> GetAccountResponse:
    req = GetAccountRequest(
            playground_id=playground_id, 
            fetch_orders=True,
            status=['filled'],
            fetch_external_id=True,
        )
    
    if orders_from_date:
        req.fromRTF3339 = f'{orders_from_date}T00:00:00Z'
        
    if orders_to_date:
        req.toRTF3339 = f'{orders_to_date}T00:00:00Z'
    
    acc = client.GetAccount(
        ctx=Context(),
        request=req
    )
    
    return acc

def print_trades(orders: List[Order]):
    closed_orders = {}
    for order in orders:
        if order.side == 'sell' or order.side == 'buy_to_cover':
            for open_order in order.closes:
                if closed_orders.get(open_order.id) is None:
                    closed_orders[open_order.id] = []
                
                closed_orders[open_order.id].append(order)
    
    for order in orders:
        if order.side == 'sell' or order.side == 'buy_to_cover':
            continue
        
        if order.status != 'filled':
            continue
        
        ts = _parse_timestamp(order.create_date).strftime('%Y-%m-%d %H:%M:%S')
        open_slippage, _ = calc_open_order_slippage(order)
        pl, open_price, close_price = _calc_realized_order_profit(order)
        
        # use closed_orders to get close_slippage 
        close_ids = []
        requested_close = 0.0
        if closed_orders.get(order.id) is not None:
            for closed_order in closed_orders[order.id]:
                close_ids.append(closed_order.external_id)
            requested_close = closed_orders[order.id][0].requested_price
        
        s = f'ts={ts} open_id={order.external_id} close_id(s)={close_ids} qty={order.quantity:.4f} open_slippage={open_slippage:.4f} side={order.side} symbol={order.symbol} requested_open={order.requested_price:.4f} open_price={open_price:.4f} requested_close={requested_close:.4f} close_price={close_price:.4f} pl={pl:.4f}'
        
        print(s)
        
def filter_orders_before(orders: List[Order], from_date: datetime) -> List[Order]:
    filtered_orders = []
    for order in orders:
        if order.side == 'sell' or order.side == 'buy_to_cover':
            for open_order in order.closes:
                if open_order.create_date < from_date:
                    filtered_orders.append(open_order)
                
    return filtered_orders

def filter_open_orders(orders: List[Order]) -> List[Order]:
    filtered_orders = []
    for order in orders:
        if order.side == 'buy' or order.side == 'sell_short':
            closed_volume = 0
            for closed_order in order.closed_by:
                closed_volume += closed_order.quantity
                
            if abs(closed_volume) < abs(order.quantity):
                filtered_orders.append(order)
            
    return filtered_orders

def collect_data(orders: List[Order], position: Position, from_date: datetime) -> dict:
    profit_list = _calc_realized_profit_list(orders)
    trade_duration_list_in_seconds = _calc_trade_duration_list_in_seconds(orders)

    gross_data = {}
    gross_data['unrealized_pl_at_open'] = calc_positions(filter_orders_before(orders, from_date))
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
    
    open_slippage = calc_open_slippage(orders)
    close_slippage = calc_close_slippage(orders)
    gross_data['open_slippage'] = open_slippage
    gross_data['close_slippage'] = close_slippage

    agg_data = {}
    agg_data['profit_factor'] = gross_data['gross_profit'] / abs(gross_data['gross_loss']) if gross_data['gross_loss'] != 0 else 'n/a'
    agg_data['realized_pl'] = calc_realized_profit(profit_list)
    agg_data['win_rate'] = gross_data['winners_count'] / gross_data['total_trades'] if gross_data['total_trades'] != 0 else 'n/a'
    agg_data['avg_trade_duration_in_minutes'] = sum(trade_duration_list_in_seconds) / len(trade_duration_list_in_seconds) / 60.0 if len(trade_duration_list_in_seconds) > 0 else 'n/a'
    agg_data['total_slippage'] = calc_total_slippage(open_slippage, close_slippage)

    return {'gross_data': gross_data, 'agg_data': agg_data}

if __name__ == '__main__':
    args = argparse.ArgumentParser()
    args.add_argument('--playground-id', type=str, required=False, help="Playground ID")
    args.add_argument('--tags', type=str, nargs='+')
    args.add_argument('--twirp-host', type=str, default='http://localhost:5051', help="twirp rpc host")
    args.add_argument('--from-date', type=str, default=None, help="start date")
    args.add_argument('--to-date', type=str, default=None, help="end date")

    args = args.parse_args()

    client = PlaygroundServiceClient(args.twirp_host, timeout=60)

    if args.playground_id:
        if args.tags:
            print('playground_id and tags are mutually exclusive')
            exit(1)
            
        account = fetch_account(client, args.playground_id, args.from_date, args.to_date)
        orders = account.orders
        positions = account.positions
        data = collect_data(orders, positions, args.from_date)
        
        print('gross data:')
        pprint(data['gross_data'])

        print('agg data:')
        pprint(data['agg_data'])
    else:
        if len(args.tags) == 0:
            print('playground_id or tags is required')
            exit(1)
            
        playground_ids = fetch_playground_ids(client, args.tags)
        
        all_accounts = []
        all_data = []
        all_orders = []
        all_orders_extended = []
        for playground_id in playground_ids:
            account = fetch_account(client, playground_id, args.from_date, args.to_date)
            orders = account.orders
            positions = account.positions
            data = collect_data(orders, positions, args.from_date)
            all_data.append(data)
            all_accounts.append(account)
            all_orders.append(orders)
            all_orders_extended.extend(orders)
            
        if len(all_data) > 1:
            aggregate_data = collect_data(all_orders_extended, positions, args.from_date)
            
            print('agg data (all playgrounds):')
            pprint(aggregate_data['agg_data'])
            
            print('gross data (all playgrounds):')
            pprint(aggregate_data['gross_data'])
            
            print('-' * 50)
            
        for account, orders, data in zip(all_accounts, all_orders, all_data):
            print('Playground ID:', account.meta.playground_id)
            print('Client ID:', account.meta.client_id)
            print('*' * 20)
            
            print('trades:')
            print_trades(orders)
            print('*' * 20)
            
            print('agg data:')
            pprint(data['agg_data'])
            print('*' * 20)

            print('gross data:')
            pprint(data['gross_data'])
            print('-' * 50)
            
        

    

    
