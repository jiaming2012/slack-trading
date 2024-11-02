import requests
from enum import Enum
from datetime import datetime, timedelta
import numpy as np
from urllib.parse import urlencode
from dataclasses import dataclass
from typing import List, Dict
from zoneinfo import ZoneInfo

import grpc
from playground_pb2 import CreatePolygonPlaygroundRequest, PlaceOrderRequest, NextTickRequest, GetAccountRequest, GetCandlesRequest
from playground_pb2_grpc import PlaygroundServiceStub

@dataclass
class Trade:
    symbol: str
    quantity: int
    price: float
    create_date: datetime
    
@dataclass
class Candle:
    open: float
    high: float
    low: float
    close: float
    volume: int
    datetime: str
    
@dataclass
class Position:
    symbol: str
    quantity: float
    cost_basis: float
    maintenance_margin: float
    pl: float
    
@dataclass
class Account:
    balance: float
    equity: float
    free_margin: float
    positions: Dict[str, Position]
    
    @property
    def pl(self) -> float:
        return sum([position.pl for position in self.positions.values()])
    
    def get_position(self, symbol) -> Position:
        return self.positions.get(symbol)
    
    def get_quantity(self, symbol) -> float:
        if symbol in self.positions:
            return self.positions[symbol].quantity
        return 0.0
    
    def get_maintenance_margin(self, symbol) -> float:
        if symbol in self.positions:
            return self.positions[symbol].maintenance_margin
        return 0.0
    
    def get_position_float(self, symbol) -> float:
        if symbol in self.positions:
            return self.positions[symbol].quantity
        return 0.0
    
    def get_pl(self, symbol) -> float:
        pl = 0
        position = self.positions.get(symbol)
        if position:
            pl = position.pl
        return pl

class OrderSide(Enum):
    BUY = 'buy'
    SELL = 'sell'
    SELL_SHORT = 'sell_short'
    BUY_TO_COVER = 'buy_to_cover'
    
class RepositorySource(Enum):
    CSV = 'csv'
    POLYGON = 'polygon'

class BacktesterPlaygroundClient:
    def __init__(self, balance: float, symbol: str, start_date: str, stop_date: str, source: RepositorySource, filename: str = None, host: str = 'http://localhost:8080', grpc_host: str = 'localhost:50051'):
        self.symbol = symbol
        self.host = host
        
        channel = grpc.insecure_channel(grpc_host)
        self.stub = PlaygroundServiceStub(channel)
        
        if source == RepositorySource.CSV:
            self.id = self.create_playground_csv(balance, symbol, start_date, stop_date, filename)
        elif source == RepositorySource.POLYGON:
            self.id = self.create_playground_polygon(balance, symbol, start_date, stop_date)
        else:
            raise Exception('Invalid source')

        self.position = None

        self.account = self.fetch_and_update_account_state()
        self.current_candle = None
        self._is_backtest_complete = False
        self._initial_timestamp = None
        self.timestamp = None
        self._tick_delta_buffer = []
        
    def flush_tick_delta_buffer(self) -> List[object]:
        buffer = self._tick_delta_buffer
        self._tick_delta_buffer = []
        return buffer
        
    def fetch_and_update_account_state(self) -> Account:
        request = GetAccountRequest(
            playground_id=self.id,
            fetch_orders=False
        )
        
        response = self.stub.GetAccount(request)
        
        acc = Account(
            balance=response.balance,
            equity=response.equity,
            free_margin=response.free_margin,
            positions={
                symbol: Position(
                    symbol=symbol,
                    quantity=position.quantity,
                    cost_basis=position.cost_basis,
                    maintenance_margin=position.maintenance_margin,
                    pl=position.pl
                ) for symbol, position in response.positions.items()
            }
        )
        
        # Update the client state
        self.position = acc.get_position_float(self.symbol)
            
        return acc
    
    def calculate_future_pl(self, trade: Trade, sl: float, tp: float) -> float:
        current_date = trade.create_date
        while True:
            future_date = current_date + timedelta(hours=1)  # use library for next day
            candles = self.fetch_candles(current_date, future_date)
            
            if len(candles) == 0:
                break
            
            for candle in candles:
                if trade.quantity > 0:
                    if candle.low <= sl:
                        return -abs(trade.quantity * (sl - trade.price))
                    elif candle.high >= tp:
                        return abs(trade.quantity * (tp - trade.price))
                elif trade.quantity < 0:
                    if candle.high >= sl:
                        return -abs(trade.quantity * (sl - trade.price))
                    elif candle.low <= tp:
                        return abs(trade.quantity * (tp - trade.price))
                
            current_date = future_date
            
        return 0
    
    def fetch_reward_from_new_trades(self, current_state, sl: float, tp: float, commission: float) -> float:
        new_trades = current_state.new_trades
        if not new_trades or len(new_trades) == 0:
            return 0
        
        reward = -commission
        
        for trade in new_trades:
            if trade['symbol'] == self.symbol:
                qty = trade['quantity']
                prc = trade['price']
            
                if qty > 0:
                    sl_prc = prc - sl
                    tp_prc = prc + tp
                elif qty < 0:
                    sl_prc = prc + sl
                    tp_prc = prc - tp
                else:
                    continue
                
                reward += self.calculate_future_pl(
                    Trade(
                        symbol=trade['symbol'],
                        quantity=trade['quantity'],
                        price=prc,
                        create_date=datetime.fromisoformat(trade['create_date'])
                    ),
                    sl_prc,
                    tp_prc
                )
        
        return reward
    
    def is_backtest_complete(self) -> bool:
        return self._is_backtest_complete
    
    def fetch_candles(self, timestampFrom: datetime, timestampTo: datetime) -> List[Candle]:
        fromStr = timestampFrom.strftime('%Y-%m-%dT%H:%M:%S%z')
        toStr = timestampTo.strftime('%Y-%m-%dT%H:%M:%S%z')
        
       # Manually insert the colon in the timezone offset
        fromStr = fromStr[:-2] + ':' + fromStr[-2:]
        toStr = toStr[:-2] + ':' + toStr[-2:]
                
        response = self.stub.GetCandles(GetCandlesRequest(
            playground_id=self.id,
            symbol=self.symbol,
            fromRTF3339=fromStr,
            toRTF3339=toStr
        ))
        
        candles_data = response.bars
        if not candles_data:
            return []
        
        candles = [
            Candle(
                open=candle.open,
                high=candle.high,
                low=candle.low,
                close=candle.close,
                volume=candle.volume,
                datetime=candle.datetime,
            ) for candle in candles_data
        ]
        
        return candles
    
    def preview_tick(self, seconds: int) -> object:
        request = NextTickRequest(
            playground_id=self.id,
            seconds=seconds,
            is_preview=True
        )
        
        response = self.stub.NextTick(request)
        
        return response
        
    def tick(self, seconds: int):
        request = NextTickRequest(
            playground_id=self.id,
            seconds=seconds,
            is_preview=False
        )
        
        new_state = self.stub.NextTick(request)
        
        new_candles = new_state.new_candles
        if new_candles and len(new_candles) > 0:
            for candle in new_candles:
                if candle.symbol == self.symbol:
                    self.current_candle = candle.bar
                    break
                
        timestamp = new_state.current_time
        if timestamp:
            if self._initial_timestamp is None:
                self._initial_timestamp = datetime.fromisoformat(timestamp)
                
            self.timestamp = datetime.fromisoformat(timestamp)
                
        self._is_backtest_complete = new_state.is_backtest_complete
        
        self.account = self.fetch_and_update_account_state()
        
        self._tick_delta_buffer.append(new_state)
                            
    def time_elapsed(self) -> timedelta:
        if self.timestamp is None:
            return timedelta(0)
        
        return self.timestamp - self._initial_timestamp
    
    def get_free_margin_over_equity(self) -> float:
        return self.account.free_margin / self.account.equity
        
    def place_order(self, symbol: str, quantity: float, side: OrderSide) -> object:
        if quantity == 0:
            return
            
        if self.get_free_margin_over_equity() < 0.4:
            if quantity > 0 and side == OrderSide.BUY:
                raise Exception('Insufficient free margin')
            elif quantity < 0 and side == OrderSide.SELL_SHORT:
                raise Exception('Insufficient free margin')
  
        request = PlaceOrderRequest(
            playground_id=self.id,
            symbol=symbol,
            asset_class='equity',
            quantity=quantity,
            side=side.value,
            type='market',
            duration='day'
        )
        
        response = self.stub.PlaceOrder(request)
        
        return response
    
    def create_playground_csv(self, balance: float, symbol: str, start_date: str, stop_date: str, filename: str) -> str:
        response = requests.post(
            f'{self.host}/playground',
            json={
                'balance': balance,
                'clock': {
                    'start': start_date,
                    'stop': stop_date
                },
                'repository': {
                    'symbol': symbol,
                    'timespan': {
                        'multiplier': 1,
                        'unit': 'minute'
                    },
                    'source': {
                        'type': 'csv',
                        'filename': filename
                    }
                }
            }
        )
        
        if response.status_code != 200:
            raise Exception(response.text)
        
        return response.json()['playground_id']
        # raise Exception('Not implemented')

    
    def create_playground_polygon(self, balance: float, symbol: str, start_date: str, stop_date: str) -> str:
        request = CreatePolygonPlaygroundRequest(
            balance=balance,
            start_date=start_date,
            stop_date=stop_date,
            symbol=symbol,
            timespan_multiplier=1,
            timespan_unit='minute',
        )

        response = self.stub.CreatePlayground(request)

        return response.id

    
if __name__ == '__main__':
    try:
        # playground_client = BacktesterPlaygroundClient(300, 'AAPL', '2021-01-04', '2021-01-31', RepositorySource.POLYGON)
        playground_client = BacktesterPlaygroundClient(300, 'AAPL', '2021-01-04', '2021-01-31', RepositorySource.CSV, filename='training_data.csv')
        
        print('playground_id: ', playground_client.id)
        
        result = playground_client.place_order('AAPL', 10, OrderSide.SELL_SHORT)
                
        playground_client.tick(6000)
        
        tick_delta = playground_client.flush_tick_delta_buffer()[0]
        
        print('tick_delta #1: ', tick_delta)
        
        invalid_orders = tick_delta.invalid_orders
        
        found_insufficient_free_margin = False
        if invalid_orders:
            for order in invalid_orders:
                if order.reject_reason and order.reject_reason.find('insufficient free margin') >= 0:
                    found_insufficient_free_margin = True
                    break
        
        print('L1: found_insufficient_free_margin: ', found_insufficient_free_margin)
        
        account = playground_client.fetch_and_update_account_state()
        
        print('L1: account: ', account)
        
        playground_client.tick(360000)
        
        tick_delta = playground_client.flush_tick_delta_buffer()[0]
        
        print('tick_delta #2: ', tick_delta)
        
        invalid_orders = tick_delta.invalid_orders
        
        found_insufficient_free_margin = False
        if invalid_orders:
            for order in invalid_orders:
                if order['reject_reason'] and order['reject_reason'].find('insufficient free margin') >= 0:
                    found_insufficient_free_margin = True
                    break
        
        print('L2: found_insufficient_free_margin: ', found_insufficient_free_margin)

        found_liquidation = False
        if tick_delta.events:
            for event in tick_delta.events:
                if event.type == 'liquidation':
                    found_liquidation = True
                    break
                
        print('L2: found_liquidation: ', found_liquidation)

        account = playground_client.fetch_and_update_account_state()
        
        print('L2: account: ', account)
        
        print('L2: position: ', playground_client.position)
        
        result = playground_client.place_order('AAPL', 3, OrderSide.SELL_SHORT)
        
        playground_client.tick(360000)
                
        tick_delta = playground_client.flush_tick_delta_buffer()[0]
        
        print('tick_delta #3: ', tick_delta)
        
        account = playground_client.fetch_and_update_account_state()
        
        print('L3: account: ', account)
        
        print('L3: position: ', playground_client.position)
        
        playground_client.tick(60000)
        
        tick_delta = playground_client.flush_tick_delta_buffer()[0]
        
        print('tick_delta #3.1: ', tick_delta)
        
        playground_client.tick(60000)
        
        tick_delta = playground_client.flush_tick_delta_buffer()[0]
        
        print('tick_delta #3.2: ', tick_delta)
        
        found_liquidation = False
        if tick_delta.events:
            for event in tick_delta.events:
                if event.type == 'liquidation':
                    found_liquidation = True
                    break
                
        print('L3.1: found_liquidation: ', found_liquidation)
        
        playground_client.tick(120000)
        
        tick_delta = playground_client.flush_tick_delta_buffer()[0]
        
        print('tick_delta #3.3: ', tick_delta)
        
        found_liquidation = False
        if tick_delta.events:
            for event in tick_delta.events:
                if event.type == 'liquidation':
                    found_liquidation = True
                    break
                
        print('L3.2: found_liquidation: ', found_liquidation)
        
        tick_delta = playground_client.tick(120000)
        
        print('tick_delta #3.4: ', tick_delta)
                
        account = playground_client.fetch_and_update_account_state()
        
        print('L4: account: ', account)
        
        print('L4: position: ', playground_client.position)
        
        candles = playground_client.fetch_candles(datetime.fromisoformat('2021-01-04T09:30:00-05:00'), datetime.fromisoformat('2021-01-04T10:31:00-05:00'))
        
        print('candles: ', candles)
                
        
    except Exception as e:
        print('Exception found: ', e)
        raise(e)