from loguru import logger
import requests
from collections import deque
from enum import Enum
from datetime import datetime, timedelta
import numpy as np
from urllib.parse import urlencode
from dataclasses import dataclass
from typing import List, Dict
from zoneinfo import ZoneInfo
from dateutil.parser import isoparse
import time

from rpc.playground_twirp import PlaygroundServiceClient
from rpc.playground_pb2 import CreatePolygonPlaygroundRequest, DeletePlaygroundRequest, GetAccountRequest, GetCandlesRequest, NextTickRequest, PlaceOrderRequest, TickDelta, GetOpenOrdersRequest, Order, AccountMeta, Bar, CreateLivePlaygroundRequest, Repository
from src.cmd.stats.playground_types import RepositorySource, OrderSide, LiveAccountType
from twirp.context import Context
from twirp.exceptions import TwirpServerException

class PlaygroundEnvironment(Enum):
    SIMULATOR = 'simulator'
    LIVE = 'live'

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
    meta: AccountMeta
    
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
    
    def get_meta(self) -> AccountMeta:
        return self.meta
    
    def get_pl(self, symbol) -> float:
        pl = 0
        position = self.positions.get(symbol)
        if position:
            pl = position.pl
        return pl
    
class PlaygroundNotFoundException(Exception):
    pass

class InvalidParametersException(Exception):
    pass

def set_nested_value(d, key1, key2, value):
    if key1 not in d:
        d[key1] = {}
    d[key1][key2] = value

def network_call_with_retry(client, request, backoff=2, max_backoff=60):
    retries = 0
    while True:
        try:
            # Attempt the twirp call
            response = client(
                ctx=Context(), 
                request=request
            )
            return response
        except TwirpServerException as e:
            retries += 1
                        
            logger.warning(f"Network call failed: {e}. Retry count {retries}. Retrying in {backoff} seconds...")
            time.sleep(backoff)
            
            if backoff < max_backoff:
                backoff = min(max_backoff, backoff * 2)  # Exponential backoff
                

class BacktesterPlaygroundClient:
    def __init__(self, req: CreatePolygonPlaygroundRequest, live_account_type: LiveAccountType, source: RepositorySource, host: str = 'http://localhost:5051', grpc_host: str = 'http://localhost:5051'):
        if len(grpc_host) > 0:
            self.host = grpc_host
        else:      
            self.host = host
            
        self.symbol = None
        for repo in req.repositories:
            self.symbol = repo.symbol
            if self.symbol is not None and repo.symbol != self.symbol:
                raise Exception('Multiple symbols found in repository')
            
        if self.symbol is None:
            raise Exception('Symbol not found in repository')

        self.client = PlaygroundServiceClient(self.host, timeout=60)

        if source == RepositorySource.CSV:
            # self.id = self.create_playground_csv(balance, symbol, start_date, stop_date, filename)
            raise Exception('CSV source not supported')
        elif source == RepositorySource.POLYGON and req.environment == PlaygroundEnvironment.SIMULATOR.value:
            self.id = self.create_playground_polygon(req)
        elif req.environment == PlaygroundEnvironment.LIVE.value:
            self.id = self.create_live_playground(req, live_account_type)
            self.next_tick_at = datetime.now()
        else:
            raise Exception(f'Invalid source {source} and environment {req.environment}')

        self.position = None

        self.account = self._fetch_and_update_account_state()
        self.current_candles = {}
        self._is_backtest_complete = False
        self._initial_timestamp = None
        self.timestamp = None
        self.trade_timestamps = []
        self._new_state_buffer: List[TickDelta] = []
        self.environment = req.environment
        
    def get_realized_profit(self) -> float:
        initial_balance = self.account.meta.initial_balance
        return self.account.balance - initial_balance
        
    def remove_from_server(self):
        request = DeletePlaygroundRequest(
            playground_id=self.id
        )
        
        try:
            network_call_with_retry(self.client.DeletePlayground, request)
        except Exception as e:
            logger.exception("Failed to connect to gRPC service (remove_on_server)")
            raise e
    
    def flush_new_state_buffer(self) -> List[TickDelta]:
        buffer = self._new_state_buffer
        self._new_state_buffer = []
        return buffer
    
    def get_current_candle(self, symbol: str, period: int) -> float:
        symbols = self.current_candles.get(symbol)
        if not symbols:
            raise Exception(f"Current bar for symbol {symbol} not found")
        
        current_bar = symbols.get(period)
        if not current_bar:
            raise Exception(f"Current bar for symbol {symbol} and period {period} not found")
        
        return current_bar
    
    def fetch_open_orders(self, symbol: str) -> List[Order]:
        request = GetOpenOrdersRequest(
            playground_id=self.id,
            symbol=symbol
        )
        
        try:
            response = network_call_with_retry(self.client.GetOpenOrders, request)
        except Exception as e:
            logger.exception("failed to connect to gRPC service (fetch_open_orders)")
            raise e
        
        return response.orders
        
    def _fetch_and_update_account_state(self) -> Account:
        request = GetAccountRequest(
            playground_id=self.id,
            fetch_orders=False
        )
        
        try:
            response = network_call_with_retry(self.client.GetAccount, request)
        except Exception as e:
            logger.exception("Failed to connect to gRPC service (fetch_and_update_account_state)")
            raise e
        
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
            },
            meta=response.meta
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
                        create_date=isoparse(trade['create_date'])
                    ),
                    sl_prc,
                    tp_prc
                )
        
        return reward
    
    def is_backtest_complete(self) -> bool:
        return self._is_backtest_complete
    
    def fetch_candles_v2(self, period_in_seconds: int, timestampFrom: datetime, timestampTo: datetime) -> List[Bar]:
        fromStr = timestampFrom.strftime('%Y-%m-%dT%H:%M:%S%z')
        toStr = timestampTo.strftime('%Y-%m-%dT%H:%M:%S%z')
        
       # Manually insert the colon in the timezone offset
        fromStr = fromStr[:-2] + ':' + fromStr[-2:]
        toStr = toStr[:-2] + ':' + toStr[-2:]
                        
        try:
            response = network_call_with_retry(self.client.GetCandles, GetCandlesRequest(
                playground_id=self.id,
                symbol=self.symbol,
                period_in_seconds=period_in_seconds,
                fromRTF3339=fromStr,
                toRTF3339=toStr
            ))
        
        except Exception as e:
            logger.exception("Failed to connect to gRPC service (fetch_candles)")
            raise e
        
        return response.bars
    
    def fetch_candles(self, period_in_seconds: int, timestampFrom: datetime, timestampTo: datetime) -> List[Candle]:
        fromStr = timestampFrom.strftime('%Y-%m-%dT%H:%M:%S%z')
        toStr = timestampTo.strftime('%Y-%m-%dT%H:%M:%S%z')
        
       # Manually insert the colon in the timezone offset
        fromStr = fromStr[:-2] + ':' + fromStr[-2:]
        toStr = toStr[:-2] + ':' + toStr[-2:]
                        
        try:
            response = network_call_with_retry(self.client.GetCandles, GetCandlesRequest(
                playground_id=self.id,
                symbol=self.symbol,
                period_in_seconds=period_in_seconds,
                fromRTF3339=fromStr,
                toRTF3339=toStr
            ))
        
        except Exception as e:
            logger.exception("Failed to connect to gRPC service (fetch_candles)")
            raise e
        
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
        
        try:
            response = network_call_with_retry(self.client.NextTick, request)
        except Exception as e:
            logger.exception("Failed to connect to gRPC service (preview_tick)")
            raise e
                
        return response
        
    def tick(self, seconds: int, raise_exception=True):
        if self.environment == PlaygroundEnvironment.LIVE.value:
            now = datetime.now()
            if now < self.next_tick_at:
                wait_period = (self.next_tick_at - now).total_seconds()
                time.sleep(wait_period)
            
            self.next_tick_at = now + timedelta(seconds=seconds)
            
        request = NextTickRequest(
            playground_id=self.id,
            seconds=seconds,
            is_preview=False
        )
        
        try:
            new_state: TickDelta = network_call_with_retry(self.client.NextTick, request)
        except Exception as e:
            logger.exception("Failed to connect to gRPC service (tick)")
            if raise_exception:
                raise e
            return None
        
        new_candles = new_state.new_candles
        if new_candles and len(new_candles) > 0:
            for candle in new_candles:
                set_nested_value(self.current_candles, candle.symbol, candle.period, candle.bar)

        timestamp = new_state.current_time
        if timestamp:
            if self._initial_timestamp is None:
                self._initial_timestamp = isoparse(timestamp)
                
            self.timestamp = isoparse(timestamp)
                
        self._is_backtest_complete = new_state.is_backtest_complete
        
        self.account = self._fetch_and_update_account_state()
        
        self._new_state_buffer.append(new_state)
                                    
    def time_elapsed(self) -> timedelta:
        if self.timestamp is None:
            return timedelta(0)
        
        return self.timestamp - self._initial_timestamp
    
    def get_free_margin_over_equity(self) -> float:
        return self.account.free_margin / self.account.equity if self.account.equity > 0 else 0
        
    def place_order(self, symbol: str, quantity: float, side: OrderSide, price=0, tag: str = "", raise_exception=True, with_tick=False) -> object:
        if quantity == 0:
            return
        
        free_margin_over_equity = self.get_free_margin_over_equity()
        if free_margin_over_equity < 0.2:
            if quantity > 0 and side == OrderSide.BUY:
                raise InvalidParametersException(f'Insufficient free margin ({free_margin_over_equity * 100}%): new long order')
            elif quantity < 0 and side == OrderSide.SELL_SHORT:
                raise InvalidParametersException(f'Insufficient free margin ({free_margin_over_equity * 100}%): new short order')
  
        request = PlaceOrderRequest(
            playground_id=self.id,
            symbol=symbol,
            asset_class='equity',
            quantity=quantity,
            side=side.value,
            type='market',
            duration='day',
            tag=tag,
            requested_price=price
        )
                
        try:
            response = network_call_with_retry(self.client.PlaceOrder, request)
            self.trade_timestamps.append(self.timestamp)
            
            if self.environment == PlaygroundEnvironment.SIMULATOR.value and with_tick:
                self.playground.tick(0, raise_exception=False)
            
            return response
        except Exception as e:
            if raise_exception:
                raise e
            return None
                    
    def create_playground_csv(self, balance: float, symbol: str, start_date: str, stop_date: str, filename: str) -> str:
        raise Exception('Not implemented')
        
    def create_live_playground(self, req: CreatePolygonPlaygroundRequest, account_type: LiveAccountType) -> str:
        try:
            liveRequest = CreateLivePlaygroundRequest(
                balance=req.balance,
                broker='tradier',
                account_type=account_type,
                repositories=req.repositories,
                environment='live'
            )
            
            response = network_call_with_retry(self.client.CreateLivePlayground, liveRequest)            
            return response.id
        except Exception as e:
            raise("Failed to create live playground:", e)

    
    def create_playground_polygon(self, req: CreatePolygonPlaygroundRequest) -> str:
        try:
            response = network_call_with_retry(self.client.CreatePlayground, req)            
            return response.id
        except Exception as e:
            raise("Failed to create playground:", e)
        
    
if __name__ == '__main__':
    try:
        playground_client = BacktesterPlaygroundClient(300, 'AAPL', '2021-01-04', '2021-01-31', RepositorySource.POLYGON)
        # playground_client = BacktesterPlaygroundClient(300, 'AAPL', '2021-01-04', '2021-01-31', RepositorySource.CSV, filename='training_data.csv')
        
        print('playground_id: ', playground_client.id)
        
        result = playground_client.place_order('AAPL', 10, OrderSide.SELL_SHORT)
                
        playground_client.tick(6000)
        
        tick_delta = playground_client.flush_new_state_buffer()[0]
        
        print('tick_delta #1: ', tick_delta)
        
        invalid_orders = tick_delta.invalid_orders
        
        found_insufficient_free_margin = False
        if invalid_orders:
            for order in invalid_orders:
                if order.reject_reason and order.reject_reason.find('insufficient free margin') >= 0:
                    found_insufficient_free_margin = True
                    break
        
        print('L1: found_insufficient_free_margin: ', found_insufficient_free_margin)
        
        account = playground_client._fetch_and_update_account_state()
        
        print('L1: account: ', account)
        
        playground_client.tick(360000)
        
        tick_delta = playground_client.flush_new_state_buffer()[0]
        
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

        account = playground_client._fetch_and_update_account_state()
        
        print('L2: account: ', account)
        
        print('L2: position: ', playground_client.position)
        
        result = playground_client.place_order('AAPL', 3, OrderSide.SELL_SHORT)
        
        playground_client.tick(360000)
                
        tick_delta = playground_client.flush_new_state_buffer()[0]
        
        print('tick_delta #3: ', tick_delta)
        
        account = playground_client._fetch_and_update_account_state()
        
        print('L3: account: ', account)
        
        print('L3: position: ', playground_client.position)
        
        playground_client.tick(60000)
        
        tick_delta = playground_client.flush_new_state_buffer()[0]
        
        print('tick_delta #3.1: ', tick_delta)
        
        playground_client.tick(60000)
        
        tick_delta = playground_client.flush_new_state_buffer()[0]
        
        print('tick_delta #3.2: ', tick_delta)
        
        found_liquidation = False
        if tick_delta.events:
            for event in tick_delta.events:
                if event.type == 'liquidation':
                    found_liquidation = True
                    break
                
        print('L3.1: found_liquidation: ', found_liquidation)
        
        playground_client.tick(120000)
        
        tick_delta = playground_client.flush_new_state_buffer()[0]
        
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
                
        account = playground_client._fetch_and_update_account_state()
        
        print('L4: account: ', account)
        
        print('L4: position: ', playground_client.position)
        
        candles = playground_client.fetch_candles(isoparse('2021-01-04T09:30:00-05:00'), isoparse('2021-01-04T10:31:00-05:00'))
        
        print('candles: ', candles)
                
        
    except Exception as e:
        print('Exception found: ', e)
        raise(e)