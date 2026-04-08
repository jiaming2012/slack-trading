import argparse
import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))
from rpc.playground_twirp import PlaygroundServiceClient
from rpc.playground_pb2 import GetAccountRequest, GetAccountResponse, GetPlaygroundsRequest, Order, Trade, AccountMeta, Bar, Position
from twirp.context import Context
from pprint import pprint
from statistics import median
from typing import List, Dict, Tuple
from datetime import datetime
from dataclasses import dataclass
from dateutil.parser import parse, ParserError
from pytz import timezone, UTC
import re
import json

@dataclass
class TradePosition:
    vwap: float
    quantity: float
    current_price: float
    pl: float

    def to_dict(self):
        return {'vwap': self.vwap, 'quantity': self.quantity, 'current_price': self.current_price, 'pl': self.pl}

class MetricsEncoder(json.JSONEncoder):
    def default(self, obj):
        if isinstance(obj, TradePosition):
            return obj.to_dict()
        return super().default(obj)
    
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
        if order.side in ['buy', 'sell_short', 'buy_to_open', 'sell_to_open']:
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
    if order.side not in ('buy', 'buy_to_open', 'sell_short', 'sell_to_open'):
        return None, 0, 0

    pl = 0
    open_price = 0
    close_prices = []

    if order.side in ['buy', 'buy_to_open']:
        open_position = _calc_trade_position(order.trades)
        open_price = open_position.vwap
        if open_price > 0:
            if getattr(order, 'class') == 'option':
                for trade in order.closed_by:
                    pl += (trade.price - open_price) * abs(trade.quantity) * 100.0
            else:
                for trade in order.closed_by:
                    pl += (trade.price - open_price) * abs(trade.quantity)

    elif order.side in ['sell_short', 'sell_to_open']:
        open_position = _calc_trade_position(order.trades)
        open_price = open_position.vwap
        if open_price > 0:
            if getattr(order, 'class') == 'option':
                for trade in order.closed_by:
                    pl += (open_price - trade.price) * trade.quantity * 100.0
            else:
                for trade in order.closed_by:
                    pl += (open_price - trade.price) * trade.quantity

    # VWAP close price (quantity-weighted average)
    total_close_qty = sum(abs(t.quantity) for t in order.closed_by)
    close_price = sum(t.price * abs(t.quantity) for t in order.closed_by) / total_close_qty if total_close_qty > 0 else 0
    return pl, open_price, close_price
                    
def _calc_realized_profits_list_2(orders: List[Order]) -> List[float]:
    pl =[]
    for o in orders:
        if o.pl is not None:
            pl.append(o.pl)
            
    return pl
    
    
def _calc_realized_profit_list(orders) -> List[float]:    
    realized_profits = []
    
    for order in orders:
        pl, _, _ = _calc_realized_order_profit(order)
        if pl is None:
            continue
        
        realized_profits.append(pl)
        
    pl_1 = sum(realized_profits)
    realized_profits_2 = _calc_realized_profits_list_2(orders)
    pl_2 = sum(realized_profits_2)
    
    assert abs(pl_1 - pl_2) < 0.001, f'pl mismatch: {pl_1} != {pl_2}'
        
    return realized_profits

def calc_positions(orders) -> Dict[str, TradePosition]:
    positions = {}
    b_start_calculation = False

    for order in orders:
        if order.side in ['buy', 'sell_short', 'buy_to_open', 'sell_to_open']:
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
        
    mark_for_deletion = []
    for symbol, p in positions.items():
        p.current_price = 0  # Placeholder for current price retrieval logic
        if p.quantity == 0:
            mark_for_deletion.append(symbol)
            
    for symbol in mark_for_deletion:
        del positions[symbol]
    
    return positions

def calc_gross_profit(profits : List[float]) -> float:
    return sum([profit for profit in profits if profit > 0])

def calc_gross_loss(profits : List[float]) -> float:
    return sum([profit for profit in profits if profit < 0])

def calc_total_orders(orders) -> int:
    return len(orders)

def calc_close_order_slippage(order) -> float:
    if order.requested_price == 0:
        return 0.0, 0.0

    if order.side in ['sell', 'sell_to_close']:
        slippage_in_points = order.requested_price - order.trades[0].price
    elif order.side in ['buy_to_cover', 'buy_to_close']:
        slippage_in_points = order.trades[0].price - order.requested_price
    else:
        return 0.0, 0.0

    slippage_in_dollars = slippage_in_points * abs(order.trades[0].quantity)
    return slippage_in_points, slippage_in_dollars

def calc_open_order_slippage(order) -> float:
    if order.side in ['buy', 'buy_to_open']:
        slippage_in_points = order.trades[0].price - order.requested_price
    elif order.side in ['sell_short', 'sell_to_open']:
        slippage_in_points = order.requested_price - order.trades[0].price
    else:
        return 0.0, 0.0

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
        if order.side in ['buy', 'buy_to_open']:
            trade_count += 1
        elif order.side in ['sell_short', 'sell_to_open']:
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

def build_trades(orders: List[Order]) -> List[dict]:
    closed_orders = {}
    for order in orders:
        if order.side in ['sell', 'sell_to_close', 'buy_to_cover', 'buy_to_close']:
            for open_order in order.closes:
                if closed_orders.get(open_order.id) is None:
                    closed_orders[open_order.id] = []

                closed_orders[open_order.id].append(order)

    trades = []
    for order in orders:
        if order.side in ['sell', 'sell_to_close', 'buy_to_cover', 'buy_to_close']:
            continue

        if order.status != 'filled':
            continue

        open_timestamp = _parse_timestamp(order.create_date)
        ts = open_timestamp.strftime('%Y-%m-%d %H:%M:%S')
        open_slippage, _ = calc_open_order_slippage(order)
        pl, open_price, close_price = _calc_realized_order_profit(order)

        # calculate weighted duration
        close_timestamps = []
        total_quantity = 0
        for trade in order.closed_by:
            close_timestamps.append((_parse_timestamp(trade.create_date), trade.quantity))
            total_quantity += trade.quantity

        duration_minutes = None
        if total_quantity != 0:
            weighted_seconds = sum(
                (ct - open_timestamp).total_seconds() * (qty / total_quantity)
                for ct, qty in close_timestamps
            )
            duration_minutes = weighted_seconds / 60.0

        # use closed_orders to get close_slippage
        close_ids = []
        requested_close = 0.0
        if closed_orders.get(order.id) is not None:
            for closed_order in closed_orders[order.id]:
                close_ids.append(closed_order.external_id)
            requested_close = closed_orders[order.id][0].requested_price

        closed_qty = sum(abs(t.quantity) for t in order.closed_by)
        partially_closed = abs(closed_qty - order.quantity) > 0.001

        trades.append({
            'ts': ts,
            'open_id': order.external_id,
            'close_ids': close_ids,
            'qty': order.quantity,
            'closed_qty': closed_qty if partially_closed else order.quantity,
            'partially_closed': partially_closed,
            'duration_minutes': duration_minutes,
            'open_slippage': open_slippage,
            'side': order.side,
            'symbol': order.symbol,
            'requested_open': order.requested_price,
            'open_price': open_price,
            'requested_close': requested_close,
            'close_price': close_price,
            'pl': pl,
        })

    return trades

def print_trades(trades: List[dict]):
    for t in trades:
        qty_str = f'{t["closed_qty"]:.4f}/{t["qty"]:.4f}' if t['partially_closed'] else f'{t["qty"]:.4f}'

        if t['duration_minutes'] is None:
            duration_str = 'n/a'
        elif t['duration_minutes'] >= 60:
            duration_str = f'{t["duration_minutes"] / 60.0:.1f}h'
        else:
            duration_str = f'{t["duration_minutes"]:.1f}m'

        s = f'ts={t["ts"]} open_id={t["open_id"]} close_id(s)={t["close_ids"]} qty={qty_str} duration={duration_str} open_slippage={t["open_slippage"]:.4f} side={t["side"]} symbol={t["symbol"]} requested_open={t["requested_open"]:.4f} open_price={t["open_price"]:.4f} requested_close={t["requested_close"]:.4f} close_price={t["close_price"]:.4f} pl={t["pl"]:.4f}'
        print(s)
        
def filter_orders_before(orders: List[Order], from_date: datetime) -> List[Order]:
    filtered_orders = []
    for order in orders:
        if order.side in ['sell', 'sell_to_close', 'buy_to_cover', 'buy_to_close']:
            for open_order in order.closes:
                if open_order.create_date < from_date:
                    filtered_orders.append(open_order)
                
    return filtered_orders

def filter_open_orders(orders: List[Order]) -> List[Order]:
    filtered_orders = []
    for order in orders:
        if order.side in ['buy', 'buy_to_open', 'sell_short', 'sell_to_open']:
            closed_volume = 0
            for closed_order in order.closed_by:
                closed_volume += closed_order.quantity
                
            if abs(closed_volume) < abs(order.quantity):
                filtered_orders.append(order)
            
    return filtered_orders

def calc_expected_value(orders: List[Order]) -> float:
    ev = 0.0
    
    for o in orders:
        if o.side not in ['buy', 'buy_to_open', 'sell_short', 'sell_to_open']:
            continue
    
        ev_str = o.attributes.get('ev', '0.0')
        if getattr(o, 'class') == 'option':
            ev += float(ev_str) * abs(o.quantity) * 100.0
        else:
            ev += float(ev_str) * abs(o.quantity)
    
    return ev    
        

def _server_positions_to_dict(server_positions) -> Dict[str, TradePosition]:
    """Convert server-provided positions to TradePosition dict."""
    positions = {}
    for symbol, pos in server_positions.items():
        positions[symbol] = TradePosition(
            vwap=pos.cost_basis,
            quantity=pos.quantity,
            current_price=pos.current_price,
            pl=pos.pl,
        )
    return positions

def collect_data(orders: List[Order], position, from_date: datetime) -> dict:
    if from_date:
        orders = filter_orders_before(orders, from_date)
    
    stock_orders = [order for order in orders if getattr(order, 'class') == 'equity']
    option_orders = [order for order in orders if getattr(order, 'class') == 'option']

    # Split server positions by asset class based on which symbols appear in each order set
    stock_symbols = {o.symbol for o in stock_orders}
    option_symbols = {o.symbol for o in option_orders}
    stock_positions = {s: p for s, p in (position or {}).items() if s in stock_symbols}
    option_positions = {s: p for s, p in (position or {}).items() if s not in stock_symbols}

    gross_data = {}
    profit_list_dict = {}
    trade_duration_list_in_seconds_dict = {}
    for orders_class, orders, class_positions in zip(
        ['stock_orders', 'option_orders'],
        [stock_orders, option_orders],
        [stock_positions, option_positions],
    ):
        profit_list = _calc_realized_profit_list(orders)
        profit_list_dict[orders_class] = profit_list
        trade_duration_list_in_seconds = _calc_trade_duration_list_in_seconds(orders)
        trade_duration_list_in_seconds_dict[orders_class] = trade_duration_list_in_seconds
        
        gross_data[orders_class] = {}
        gross_data[orders_class]['total_orders'] = calc_total_orders(orders)
        gross_data[orders_class]['total_trades'] = calc_total_trades(orders)
        gross_data[orders_class]['gross_profit'] = calc_gross_profit(profit_list)
        gross_data[orders_class]['gross_loss'] = calc_gross_loss(profit_list)
        gross_data[orders_class]['winners_count'] = calc_winners_count(profit_list)
        gross_data[orders_class]['losers_count'] = calc_losers_count(profit_list)
        gross_data[orders_class]['breakeven_count'] = calc_breakeven_count(profit_list)
        gross_data[orders_class]['avg_profit'] = calc_avg_profit(profit_list)
        gross_data[orders_class]['avg_loss'] = calc_avg_loss(profit_list)
        gross_data[orders_class]['trade_duration_in_minutes'] = {
            'min': min(trade_duration_list_in_seconds) / 60.0 if len(trade_duration_list_in_seconds) > 0 else 'n/a',
            'max': max(trade_duration_list_in_seconds) / 60.0 if len(trade_duration_list_in_seconds) > 0 else 'n/a',
            'avg': sum(trade_duration_list_in_seconds) / len(trade_duration_list_in_seconds) / 60.0 if len(trade_duration_list_in_seconds) > 0 else 'n/a',
            'median': median(trade_duration_list_in_seconds) / 60.0 if len(trade_duration_list_in_seconds) > 0 else 'n/a',
        }
        gross_data[orders_class]['positions'] = _server_positions_to_dict(class_positions) if class_positions else calc_positions(orders)
        gross_data[orders_class]['open_slippage'] = calc_open_slippage(orders)
        gross_data[orders_class]['close_slippage'] = calc_close_slippage(orders)

    agg_data = {}
    agg_data['total_realized_pl'] = calc_realized_profit(profit_list_dict['stock_orders']) + calc_realized_profit(profit_list_dict['option_orders'])
    agg_data['stock_profit_factor'] = gross_data['stock_orders']['gross_profit'] / abs(gross_data['stock_orders']['gross_loss']) if gross_data['stock_orders']['gross_loss'] != 0 else 'n/a'
    agg_data['stock_realized_pl'] = calc_realized_profit(profit_list_dict['stock_orders'])
    agg_data['stock_win_rate'] = gross_data['stock_orders']['winners_count'] / gross_data['stock_orders']['total_trades'] if gross_data['stock_orders']['total_trades'] != 0 else 'n/a'
    agg_data['stock_avg_trade_duration_in_minutes'] = sum(trade_duration_list_in_seconds_dict['stock_orders']) / len(trade_duration_list_in_seconds_dict['stock_orders']) / 60.0 if len(trade_duration_list_in_seconds_dict['stock_orders']) > 0 else 'n/a'
    agg_data['stock_total_slippage'] = calc_total_slippage(gross_data['stock_orders']['open_slippage'], gross_data['stock_orders']['close_slippage'])
    agg_data['stock_expected_value'] = calc_expected_value(stock_orders)
    agg_data['option_profit_factor'] = gross_data['option_orders']['gross_profit'] / abs(gross_data['option_orders']['gross_loss']) if gross_data['option_orders']['gross_loss'] != 0 else 'n/a'
    agg_data['option_realized_pl'] = calc_realized_profit(profit_list_dict['option_orders'])
    agg_data['option_win_rate'] = gross_data['option_orders']['winners_count'] / gross_data['option_orders']['total_trades'] if gross_data['option_orders']['total_trades'] != 0 else 'n/a'
    agg_data['option_avg_trade_duration_in_minutes'] = sum(trade_duration_list_in_seconds_dict['option_orders']) / len(trade_duration_list_in_seconds_dict['option_orders']) / 60.0 if len(trade_duration_list_in_seconds_dict['option_orders']) > 0 else 'n/a'
    agg_data['option_total_slippage'] = calc_total_slippage(gross_data['option_orders']['open_slippage'], gross_data['option_orders']['close_slippage'])
    agg_data['option_expected_value'] = calc_expected_value(option_orders)

    stock_unrealized_pl = sum(pos.pl for pos in gross_data['stock_orders']['positions'].values())
    option_unrealized_pl = sum(pos.pl for pos in gross_data['option_orders']['positions'].values())
    agg_data['stock_unrealized_pl'] = stock_unrealized_pl
    agg_data['option_unrealized_pl'] = option_unrealized_pl
    agg_data['total_unrealized_pl'] = stock_unrealized_pl + option_unrealized_pl
    agg_data['total_stock_pl'] = agg_data['stock_realized_pl'] + stock_unrealized_pl
    agg_data['total_option_pl'] = agg_data['option_realized_pl'] + option_unrealized_pl

    return {'gross_data': gross_data, 'agg_data': agg_data}

if __name__ == '__main__':
    args = argparse.ArgumentParser()
    args.add_argument('--playground-id', type=str, required=False, help="Playground ID")
    args.add_argument('--tags', type=str, nargs='+')
    args.add_argument('--twirp-host', type=str, default='http://localhost:5051', help="twirp rpc host")
    args.add_argument('--from-date', type=str, default=None, help="start date")
    args.add_argument('--to-date', type=str, default=None, help="end date")
    args.add_argument('--format', type=str, choices=['text', 'json'], default='text', help="output format")

    args = args.parse_args()

    twirp_host = args.twirp_host
    if not twirp_host.startswith('http://') and not twirp_host.startswith('https://'):
        twirp_host = f'http://{twirp_host}'

    client = PlaygroundServiceClient(twirp_host, timeout=60)

    all_accounts = []
    all_data = []
    all_orders = []
    all_orders_extended = []
    if args.playground_id:
        if args.tags:
            print('playground_id and tags are mutually exclusive')
            exit(1)
            
        account = fetch_account(client, args.playground_id, args.from_date, args.to_date)
        orders = account.orders
        positions = account.positions
        data = collect_data(orders, positions, args.from_date)
        all_data.append(data)
        all_accounts.append(account)
        all_orders.append(orders)
        all_orders_extended.extend(orders)
        
        for account, orders, data in zip(all_accounts, all_orders, all_data):
            trades = build_trades(orders)
            combined = {**data['agg_data'], **data['gross_data']}

            if args.format == 'json':
                output = {
                    'playground_id': account.meta.playground_id,
                    'client_id': account.meta.client_id,
                    'trades': trades,
                    'metrics': combined,
                }
                print(json.dumps(output, indent=2, cls=MetricsEncoder))
            else:
                print(f'Playground: {account.meta.playground_id}')
                if account.meta.client_id:
                    print(f'Client:     {account.meta.client_id}')
                print()
                print_trades(trades)
                print()
                pprint(combined)
                print()

    else:
        if not args.tags:
            print('playground_id or tags is required')
            exit(1)
            
        playground_ids = fetch_playground_ids(client, args.tags)
        
        
        for playground_id in playground_ids:
            account = fetch_account(client, playground_id, args.from_date, args.to_date)
            orders = account.orders
            positions = account.positions
            data = collect_data(orders, positions, args.from_date)
            all_data.append(data)
            all_accounts.append(account)
            all_orders.append(orders)
            all_orders_extended.extend(orders)
            
        if args.format == 'json':
            results = []
            for account, orders, data in zip(all_accounts, all_orders, all_data):
                trades = build_trades(orders)
                combined = {**data['agg_data'], **data['gross_data']}
                results.append({
                    'playground_id': account.meta.playground_id,
                    'client_id': account.meta.client_id,
                    'trades': trades,
                    'metrics': combined,
                })

            output = {'playgrounds': results}
            if len(all_data) > 1:
                aggregate_data = collect_data(all_orders_extended, positions, args.from_date)
                output['aggregate'] = {**aggregate_data['agg_data'], **aggregate_data['gross_data']}

            print(json.dumps(output, indent=2, cls=MetricsEncoder))
        else:
            if len(all_data) > 1:
                aggregate_data = collect_data(all_orders_extended, positions, args.from_date)

                print('=== All Playgrounds ===')
                combined = {**aggregate_data['agg_data'], **aggregate_data['gross_data']}
                pprint(combined)
                print()

            for account, orders, data in zip(all_accounts, all_orders, all_data):
                trades = build_trades(orders)
                print(f'Playground: {account.meta.playground_id}')
                if account.meta.client_id:
                    print(f'Client:     {account.meta.client_id}')
                print()
                print_trades(trades)
                print()
                combined = {**data['agg_data'], **data['gross_data']}
                pprint(combined)
                print()
            
        

    

    
