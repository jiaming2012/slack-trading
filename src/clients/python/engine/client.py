from loguru import logger as loguru_logger
import requests
from collections import deque
from enum import Enum
from datetime import datetime, timedelta, timezone
import numpy as np
from urllib.parse import urlencode
from dataclasses import dataclass
from typing import List, Dict
from zoneinfo import ZoneInfo
from dateutil.parser import isoparse
from utils import get_timespan_unit
import time
from time import perf_counter
import uuid


from rpc.playground_twirp import PlaygroundServiceClient
from google.protobuf.timestamp_pb2 import Timestamp
from rpc.playground_pb2 import CreatePolygonPlaygroundRequest, DeletePlaygroundRequest, GetAccountRequest, GetCandlesRequest, NextTickRequest, PlaceOrderRequest, TickDelta, GetOpenOrdersRequest, Order, AccountMeta, Bar, CreateLivePlaygroundRequest, Repository, Candle as pb_Candle, WriteSignalRequest, GetSignalsRequest, GetProcessedSignalsRequest
from engine.types import RepositorySource, OrderSide, LiveAccountType
from twirp.context import Context
from twirp.exceptions import TwirpServerException

MAX_RETRIES = 6

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
    current_price: float
    
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

class PlaceOrderSideNotAllowedException(Exception):
    pass

def set_nested_value(d, key1, key2, value):
    if key1 not in d:
        d[key1] = {}
    d[key1][key2] = value            
                

class BacktesterPlaygroundClient:
    def is_retryable_exception(self, e):
        """
        Check if the exception is retryable.
        """
        if isinstance(e, TwirpServerException):
            # Check for specific error codes that are retryable
            if e.message.find('volume to close exceeds open volume') >= 0:
                return False
            
            if e.message.find('PlaceOrder: side not allowed') >= 0:
                e.detailed_error = PlaceOrderSideNotAllowedException(e.message)
                return False
            
        return True
    
    def fetch_ladder(self, request):        
        try:
            response = self.network_call_with_retry('fetch_ladder', self.client.GetOptionsLadder, request)
        except Exception as e:
            self.logger.exception("failed to connect to gRPC service (fetch_ladder)", timestamp=self.timestamp)
            raise e
        
        return response
        
    def network_call_with_retry(self, caller, client, request, backoff=2, max_backoff=60):
        _start = perf_counter()
        try:
            return self._network_call_with_retry_inner(caller, client, request, backoff, max_backoff)
        finally:
            if getattr(self, 'profiler', None) is not None:
                self.profiler.record(caller, perf_counter() - _start)

    def _network_call_with_retry_inner(self, caller, client, request, backoff, max_backoff):
        retries = 0
        headers = {}
        while True:
            try:
                response = client(
                    ctx=Context(headers=headers),
                    request=request
                )
                return response
            except TwirpServerException as e:
                retries += 1

                if not self.is_retryable_exception(e):
                    self.logger.error(f"{caller} network call failed with non-retryable exception: {e}. Giving up.")
                    if e.detailed_error:
                        raise e.detailed_error
                    raise e

                if retries > MAX_RETRIES:
                    self.logger.error(f"{caller} network call failed after {retries} retries. Giving up.")
                    raise e

                self.logger.error(f"{caller} network call failed: {e}. Retry count {retries}. Retrying in {backoff} seconds...")
                time.sleep(backoff)

                if backoff < max_backoff:
                    backoff = min(max_backoff, backoff * 2)
                    
    def set_current_candle(self, symbol: str, period: int, bar: Bar):        
        set_nested_value(self.current_candles, symbol, period, bar)
                
    def fetch_closest_bar(self, symbol: str, period: int, timestamp: datetime) -> Bar:
        fromTimestamp = timestamp - timedelta(days=3)
        toTimestamp = timestamp + timedelta(minutes=1)
        
        bars = self.fetch_candles_v3(symbol, period, fromTimestamp, toTimestamp)
        if not bars or len(bars) == 0:
            raise Exception(f"No bars found for symbol {symbol} and period {period}")
        
        for bar in reversed(bars):
            bar_time = isoparse(bar.datetime).astimezone(ZoneInfo("America/New_York"))
            if bar_time <= timestamp:
                return bar
                
    def fetch_most_recent_bar(self, symbol: str, period: int, now: datetime) -> Bar:
        # Calculate three days ago
        three_days_ago = now - timedelta(days=3)
        
        bars = self.fetch_candles_v3(symbol, period, three_days_ago)
        if not bars or len(bars) == 0:
            raise Exception(f"No bars found for symbol {symbol} and period {period}")
        
        return bars[-1]
    
    def get_option_positions(self) -> Dict[str, Position]:
        option_positions = {}
        for symbol, position in self.account.positions.items():
            if symbol.startswith('O:') and (('C' in symbol) or ('P' in symbol)):
                option_positions[symbol] = position
        return option_positions
    
    def get_options_quantity(self, underlying_symbol: str) -> int:
        qty = 0
        for symbol, position in self.account.positions.items():
            if symbol.startswith(f'O:{underlying_symbol}') and (('C' in symbol) or ('P' in symbol)):
                qty += int(position.quantity)
        return qty
    
    def get_repository_seconds(self, tf: str = 'ltf') -> Repository:            
        if len(self.repositories) == 0:
            raise Exception('No repositories found')
        
        if tf == 'ltf':
            ltf = self.repositories[0].timespan_multiplier * get_timespan_unit(self.repositories[0].timespan_unit)
            for i, repo in enumerate(self.repositories):
                unit_multiplier = get_timespan_unit(repo.timespan_unit)
                val = repo.timespan_multiplier
                if val * unit_multiplier < ltf:
                    self.logger.debug(f"{unit_multiplier * val} < {ltf}:ltf for {repo.symbol}")
                    ltf = val * unit_multiplier
                    
            return ltf
        
        elif tf == 'htf':
            htf = self.repositories[0].timespan_multiplier * get_timespan_unit(self.repositories[0].timespan_unit)
            for i, repo in enumerate(self.repositories):                
                unit_multiplier = get_timespan_unit(repo.timespan_unit)
                val = repo.timespan_multiplier
                if val * unit_multiplier > htf:
                    self.logger.debug(f"{unit_multiplier * val} > {htf}:htf for {repo.symbol}")
                    htf = val * unit_multiplier
                    
            return htf
        
        elif tf == 'htf_daily':
            for i, repo in enumerate(self.repositories):
                if repo.timespan_unit == 'day':
                    unit_multiplier = get_timespan_unit(repo.timespan_unit)
                    val = repo.timespan_multiplier
                    htf_daily = val * unit_multiplier
                        
                    return htf_daily
            
            raise Exception('No daily repository found')
        
        elif tf == 'htf_weekly':
            for i, repo in enumerate(self.repositories):
                if repo.timespan_unit == 'week':                
                    unit_multiplier = get_timespan_unit(repo.timespan_unit)
                    val = repo.timespan_multiplier
                    htf_weekly = val * unit_multiplier
                        
                    return htf_weekly
            
            raise Exception('No weekly repository found')
        
        else:
            raise Exception(f'Invalid timespan multiplier {tf}')
        
                
    def __init__(self, req: CreatePolygonPlaygroundRequest, live_account_type: LiveAccountType, source: RepositorySource, logger, twirp_host: str = 'http://localhost:5051'):
        self.host = twirp_host 
        
        if not isinstance(logger, loguru_logger.__class__):
            raise Exception('Invalid logger: must be an instance of loguru logger')
        
        self.logger = logger
        self.repositories = req.repositories

        self.client = PlaygroundServiceClient(self.host, timeout=600)
        self.ltf_seconds = self.get_repository_seconds('ltf')
        self.htf_seconds = self.get_repository_seconds('htf')
        # self.htf_seconds_daily = self.get_repository_seconds('htf_daily')
        # self.htf_seconds_weekly = self.get_repository_seconds('htf_weekly')

        if source == RepositorySource.CSV:
            # self.id = self.create_playground_csv(balance, symbol, start_date, stop_date, filename)
            raise Exception('CSV source not supported')
        elif source == RepositorySource.POLYGON and req.environment == PlaygroundEnvironment.SIMULATOR.value:
            self.id = self.create_playground_polygon(req)
            
            resp = self.preview_tick()
            self.timestamp = isoparse(resp.current_time).astimezone(ZoneInfo("America/New_York"))
            self._initial_timestamp = self.timestamp
            
        elif req.environment == PlaygroundEnvironment.LIVE.value:
            self.id = self.create_live_playground(req, live_account_type)
            
            now = datetime.now(ZoneInfo("America/New_York"))
            self.next_tick_at = now
            self.timestamp = now
            self._initial_timestamp = now
            
        else:
            raise Exception(f'Invalid source {source} and environment {req.environment}')

        self.account = self._fetch_and_update_account_state()
        self._is_backtest_complete = False
        self.trade_timestamps = []
        self._new_state_buffer: List[TickDelta] = []
        self.environment = req.environment
        self.client_id = getattr(req, 'client_id', '') or ''
        self.current_candles = {}
        self.profiler = None  # Set externally via playground.profiler = RPCProfiler()
        self._signal_callback = None  # Set externally via playground._signal_callback = strategy.on_signal
        
        current_ltf_candle = self.fetch_most_recent_bar(req.repositories[0].symbol, self.ltf_seconds, self.timestamp)
        set_nested_value(self.current_candles, req.repositories[0].symbol, self.ltf_seconds, current_ltf_candle)   
        
        current_htf_candle = self.fetch_most_recent_bar(req.repositories[0].symbol, self.htf_seconds, self.timestamp)
        set_nested_value(self.current_candles, req.repositories[0].symbol, self.htf_seconds, current_htf_candle)
        
    def get_realized_profit(self) -> float:
        initial_balance = self.account.meta.initial_balance
        return self.account.balance - initial_balance
        
    def remove_from_server(self):
        request = DeletePlaygroundRequest(
            playground_id=self.id
        )
        
        try:
            self.network_call_with_retry('remove_from_server', self.client.DeletePlayground, request)
        except Exception as e:
            self.logger.exception("Failed to connect to gRPC service (remove_on_server)", timestamp=self.timestamp)
            raise e
    
    def flush_new_state_buffer(self) -> List[TickDelta]:
        buffer = self._new_state_buffer
        self._new_state_buffer = []
        return buffer
    
    def get_current_candle(self, symbol: str, period: int) -> Bar:
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
            response = self.network_call_with_retry('fetch_open_orders', self.client.GetOpenOrders, request)
        except Exception as e:
            self.logger.exception("failed to connect to gRPC service (fetch_open_orders)", timestamp=self.timestamp)
            raise e
        
        return response.orders
    
    def fetch_orders(self) -> List[Order]:
        request = GetAccountRequest(
            playground_id=self.id,
            fetch_orders=True
        )
        
        try:
            response = self.network_call_with_retry('fetch_orders', self.client.GetAccount, request)
        except Exception as e:
            self.logger.exception("Failed to connect to gRPC service (fetch_and_update_account_state)", timestamp=self.timestamp)
            raise e
        
        return response.orders
        
    def _fetch_and_update_account_state(self) -> Account:
        request = GetAccountRequest(
            playground_id=self.id,
            fetch_orders=False
        )
        
        try:
            response = self.network_call_with_retry('_fetch_and_update_account_state', self.client.GetAccount, request)
        except Exception as e:
            self.logger.exception("Failed to connect to gRPC service (fetch_and_update_account_state)", timestamp=self.timestamp)
            raise e
                
        positions = {}
        for k, v in response.positions.items():
            positions[k] = Position(
                symbol=k,
                quantity=v.quantity,
                cost_basis=v.cost_basis,
                maintenance_margin=v.maintenance_margin,
                current_price=v.current_price,
                pl=v.pl
            )
        
        acc = Account(
            balance=response.balance,
            equity=response.equity,
            free_margin=response.free_margin,
            positions=positions,
            meta=response.meta
        )
            
        return acc
    
    # def calculate_future_pl(self, trade: Trade, sl: float, tp: float) -> float:
    #     current_date = trade.create_date
    #     while True:
    #         future_date = current_date + timedelta(hours=1)  # use library for next day
    #         candles = self.fetch_candles(current_date, future_date)
            
    #         if len(candles) == 0:
    #             break
            
    #         for candle in candles:
    #             if trade.quantity > 0:
    #                 if candle.low <= sl:
    #                     return -abs(trade.quantity * (sl - trade.price))
    #                 elif candle.high >= tp:
    #                     return abs(trade.quantity * (tp - trade.price))
    #             elif trade.quantity < 0:
    #                 if candle.high >= sl:
    #                     return -abs(trade.quantity * (sl - trade.price))
    #                 elif candle.low <= tp:
    #                     return abs(trade.quantity * (tp - trade.price))
                
    #         current_date = future_date
            
    #     return 0
    
    def fetch_reward_from_new_trades(self, current_state, sl: float, tp: float, commission: float) -> float:
        raise NotImplementedError("This method has been deprecated and is not implemented in the base class")
        
        # new_trades = current_state.new_trades
        # if not new_trades or len(new_trades) == 0:
        #     return 0
        
        # reward = -commission
        
        # for trade in new_trades:
        #     if trade['symbol'] == self.symbol:
        #         qty = trade['quantity']
        #         prc = trade['price']
            
        #         if qty > 0:
        #             sl_prc = prc - sl
        #             tp_prc = prc + tp
        #         elif qty < 0:
        #             sl_prc = prc + sl
        #             tp_prc = prc - tp
        #         else:
        #             continue
                
        #         reward += self.calculate_future_pl(
        #             Trade(
        #                 symbol=trade['symbol'],
        #                 quantity=trade['quantity'],
        #                 price=prc,
        #                 create_date=isoparse(trade['create_date'])
        #             ),
        #             sl_prc,
        #             tp_prc
        #         )
        
        # return reward
    
    def is_backtest_complete(self) -> bool:
        return self._is_backtest_complete
    
    def fetch_candles_v3(self, symbol: str, period_in_seconds: int, timestampFrom: datetime, timestampTo: datetime = None, calculate_is_extended_hours: bool = False) -> List[Bar]:
        '''
        This version is used bc timestamps created from python doesn't work with v2
        '''
        timestampFromUtc = timestampFrom.astimezone(ZoneInfo('UTC'))
        fromStr = timestampFromUtc.strftime("%Y-%m-%dT%H:%M:%S") + "Z"
                        
        req = GetCandlesRequest(
                playground_id=self.id,
                symbol=symbol,
                period_in_seconds=period_in_seconds,
                fromRTF3339=fromStr            
            )
    
        if timestampTo is not None:
            timestampToUtc = timestampTo.astimezone(ZoneInfo('UTC'))
            toStr = timestampToUtc.strftime("%Y-%m-%dT%H:%M:%S") + "Z"
            req.toRTF3339 = toStr
            
        if calculate_is_extended_hours:
            req.calculate_is_extended_hours = True
                        
        try:
            response = self.network_call_with_retry('fetch_candles_v3', self.client.GetCandlesFromRepo, req)
        
        except Exception as e:
            self.logger.exception("Failed to connect to gRPC service (fetch_candles)", timestamp=self.timestamp)
            raise e
        
        return response.bars
    
    def fetch_candles_v2(self, symbol: str, period_in_seconds: int, timestampFrom: datetime, timestampTo: datetime = None) -> List[Bar]:
        fromStr = timestampFrom.strftime('%Y-%m-%dT%H:%M:%S%z')
        
       # Manually insert the colon in the timezone offset
        fromStr = fromStr[:-2] + ':' + fromStr[-2:]        
                        
        req = GetCandlesRequest(
                playground_id=self.id,
                symbol=symbol,
                period_in_seconds=period_in_seconds,
                fromRTF3339=fromStr
            )
    
        if timestampTo is not None:
            toStr = timestampTo.strftime('%Y-%m-%dT%H:%M:%S%z')
            
            # Manually insert the colon in the timezone offset
            toStr = toStr[:-2] + ':' + toStr[-2:]
            
            req.toRTF3339 = toStr
                        
        try:
            response = self.network_call_with_retry('fetch_candles_v2', self.client.GetCandlesFromRepo, req)
        
        except Exception as e:
            self.logger.exception("Failed to connect to gRPC service (fetch_candles)", timestamp=self.timestamp)
            raise e
        
        return response.bars
    
    def _wait_until_next_tick(self, seconds: int, on_wait, on_wait_interval: int):
        """Absorb real time behind the tick() interface.

        No-op in simulation. Otherwise sleeps until ``next_tick_at``,
        invoking ``on_wait`` at most every ``on_wait_interval`` seconds so
        callers can surface liveness without owning the clock.
        """
        if self.environment != PlaygroundEnvironment.LIVE.value:
            return

        started_at = datetime.now(ZoneInfo("America/New_York"))
        now = started_at
        while now < self.next_tick_at:
            remaining = (self.next_tick_at - now).total_seconds()
            if on_wait is not None:
                time.sleep(min(remaining, on_wait_interval))
                on_wait()
            else:
                time.sleep(remaining)
            now = datetime.now(ZoneInfo("America/New_York"))

        self.next_tick_at = started_at + timedelta(seconds=seconds)

    def preview_tick(self, seconds: int) -> object:
        request = NextTickRequest(
            playground_id=self.id,
            seconds=seconds,
            is_preview=True
        )
        
        try:
            response = self.network_call_with_retry('preview_tick', self.client.NextTick, request)
        except Exception as e:
            self.logger.exception("Failed to connect to gRPC service (preview_tick)", timestamp=self.timestamp)
            raise e
                
        return response
        
    # PERF TODO (Phase 4): Add a BatchTick RPC to advance multiple ticks in a
    # single call when no signal processing is needed. This would reduce the number
    # of RPC round-trips during long stretches without signals.
    def tick(self, seconds: int, raise_exception=True, fetch_account: bool = True, on_wait=None, on_wait_interval: int = 30):
        """Advance the playground and block until the next candle exists.

        The blocking contract (ADR-0002): returns instantly in simulation,
        waits out real time otherwise. Strategies never sleep, poll, or read
        the wall clock — pass ``on_wait`` to be notified (at most every
        ``on_wait_interval`` seconds) while the client waits, e.g. to print
        a status line.
        """
        self._wait_until_next_tick(seconds, on_wait, on_wait_interval)

        request = NextTickRequest(
            playground_id=self.id,
            seconds=seconds,
            is_preview=False,
            request_id=str(uuid.uuid4()),
        )

        try:
            new_state: TickDelta = self.network_call_with_retry('tick', self.client.NextTick, request)
        except Exception as e:
            self.logger.exception("Failed to connect to gRPC service (tick)", timestamp=self.timestamp)
            if raise_exception:
                raise e
            return None

        new_candles = new_state.new_candles
        if new_candles and len(new_candles) > 0:
            for candle in new_candles:
                set_nested_value(self.current_candles, candle.symbol, candle.period, candle.bar)

        timestamp = new_state.current_time
        if timestamp:
            self.timestamp = isoparse(timestamp)
            self.timestamp = self.timestamp.astimezone(ZoneInfo("America/New_York"))

        self._is_backtest_complete = new_state.is_backtest_complete

        # Use account state embedded in the TickDelta response (no extra RPC).
        if fetch_account:
            positions = {}
            for k, v in new_state.positions.items():
                positions[k] = Position(
                    symbol=k,
                    quantity=v.quantity,
                    cost_basis=v.cost_basis,
                    maintenance_margin=v.maintenance_margin,
                    current_price=v.current_price,
                    pl=v.pl,
                )
            self.account = Account(
                balance=new_state.balance,
                equity=new_state.equity,
                free_margin=new_state.free_margin,
                positions=positions,
                meta=self.account.meta if self.account else None,
            )

        # Dispatch new signals to strategy via callback (D-03)
        if self._signal_callback is not None:
            new_signals = new_state.new_signals
            if new_signals:
                for signal in new_signals:
                    self._signal_callback(signal)

        self._new_state_buffer.append(new_state)
                                    
    def time_elapsed(self) -> timedelta:
        if self.timestamp is None:
            return timedelta(0)

        return self.timestamp - self._initial_timestamp
    
    def get_free_margin_over_equity(self) -> float:
        return self.account.free_margin / self.account.equity if self.account.equity > 0 else 0
        
    def place_order(self, symbol: str, quantity: float, side: OrderSide, asset_class: str, price=0, tag: str = "", close_order_id: str = None, raise_exception=True, with_tick=False, sl: float=None, client_request_id: str=None, attributes=None, signal_id: str = None) -> object:
        if quantity == 0:
            return
        
        free_margin_over_equity = self.get_free_margin_over_equity()
        if free_margin_over_equity < 0.2:
            if quantity > 0 and side in [OrderSide.BUY, OrderSide.BUY_TO_OPEN]:
                raise InvalidParametersException(f'Insufficient free margin (={free_margin_over_equity * 100:.2f}%) to place long order')
            elif quantity < 0 and side in [OrderSide.SELL_SHORT, OrderSide.SELL_TO_OPEN]:
                raise InvalidParametersException(f'Insufficient free margin (={free_margin_over_equity * 100:.2f}%) to place short order')
  
        if client_request_id is None:
            client_request_id = str(uuid.uuid4())
  
        request = PlaceOrderRequest(
            playground_id=self.id,
            symbol=symbol,
            asset_class=asset_class,
            quantity=quantity,
            side=side.value,
            type='market',
            duration='day',
            tag=tag,
            requested_price=price,
            client_request_id=client_request_id,
        )
        
        if sl is not None:
            request.sl = sl
        
        if close_order_id:
            request.close_order_id = close_order_id
            
        if attributes:
            for k, v in attributes.items():
                request.attributes[k] = v

        if signal_id is not None:
            request.signal_id = signal_id

        try:
            self.logger.info(f"PlaceOrder: {request.side} {request.quantity}x {request.symbol} @ {request.requested_price} [{request.tag or 'no-tag'}]", trading_operation='place_order', timestamp=self.timestamp)
            response = self.network_call_with_retry('place_order', self.client.PlaceOrder, request)
            self.trade_timestamps.append(self.timestamp)
            
            if self.environment == PlaygroundEnvironment.SIMULATOR.value:
                if with_tick:
                    self.tick(0, raise_exception=True)
            else:
                self.logger.info(f"environment={self.environment} Placing order without tick: {request}", trading_operation='place_order', timestamp=self.timestamp)
                self.logger.info("sleeping for 3 seconds ...")
                time.sleep(3)
                self.logger.info("waking up")
            
            return response
        
        except Exception as e:
            if raise_exception:
                raise e
            return None
                    
    def write_signal(self, name: str, symbol: str, timestamp: datetime, attributes: dict = None) -> str:
        """Produce a global TradeSignal via WriteSignal RPC. Returns signal_id (UUID string)."""
        ts = Timestamp()
        ts.FromDatetime(timestamp)

        req = WriteSignalRequest(
            name=name,
            symbol=symbol,
            timestamp=ts,
            attributes={k: str(v) for k, v in (attributes or {}).items()},
        )

        response = self.network_call_with_retry('write_signal', self.client.WriteSignal, req)
        return response.signal_id

    def get_signals(self, name: str = None, symbol: str = None, start_time: datetime = None, end_time: datetime = None) -> list:
        """Query all global signals with optional filters. Returns list of TradeSignalProto."""
        req = GetSignalsRequest()

        if name is not None:
            req.name = name
        if symbol is not None:
            req.symbol = symbol
        if start_time is not None:
            ts = Timestamp()
            ts.FromDatetime(start_time)
            req.start_time.CopyFrom(ts)
        if end_time is not None:
            ts = Timestamp()
            ts.FromDatetime(end_time)
            req.end_time.CopyFrom(ts)

        response = self.network_call_with_retry('get_signals', self.client.GetSignals, req)
        return list(response.signals)

    def get_processed_signals(self, playground_id: str = None, name: str = None, symbol: str = None, start_time: datetime = None, end_time: datetime = None) -> list:
        """Query signals consumed by a specific playground. Returns list of TradeSignalProto."""
        pid = playground_id or self.id

        req = GetProcessedSignalsRequest(playground_id=pid)

        if name is not None:
            req.name = name
        if symbol is not None:
            req.symbol = symbol
        if start_time is not None:
            ts = Timestamp()
            ts.FromDatetime(start_time)
            req.start_time.CopyFrom(ts)
        if end_time is not None:
            ts = Timestamp()
            ts.FromDatetime(end_time)
            req.end_time.CopyFrom(ts)

        response = self.network_call_with_retry('get_processed_signals', self.client.GetProcessedSignals, req)
        return list(response.signals)

    def create_playground_csv(self, balance: float, symbol: str, start_date: str, stop_date: str, filename: str) -> str:
        raise Exception('Not implemented')
        
    def preview_tick(self, seconds: int = 0):
        try:
            req = NextTickRequest(
                    playground_id=self.id,
                    seconds=seconds,
                    is_preview=True,
                    request_id=str(uuid.uuid4()),
                )
            
            response = self.network_call_with_retry('preview_tick', self.client.NextTick, req)
            return response
        except Exception as e:
            raise Exception("Failed to preview tick:", e)
    
    def create_live_playground(self, req: CreatePolygonPlaygroundRequest, account_type: LiveAccountType) -> str:
        try:
            liveRequest = CreateLivePlaygroundRequest(
                balance=req.balance,
                broker='tradier',
                account_type=account_type.value,
                repositories=req.repositories,
                environment='live',
                tags=req.tags
            )
            
            if len(req.client_id) > 0:
                liveRequest.client_id = req.client_id
            
            response = self.network_call_with_retry('create_live_playground', self.client.CreateLivePlayground, liveRequest)            
            return response.id
        except Exception as e:
            raise Exception("Failed to create live playground:", e)

    
    def create_playground_polygon(self, req: CreatePolygonPlaygroundRequest) -> str:
        try:
            response = self.network_call_with_retry('create_playground_polygon', self.client.CreatePlayground, req)            
            return response.id
        except Exception as e:
            raise Exception("Failed to create playground:", e)
        
    
if __name__ == '__main__':
    try:
        playground_client = BacktesterPlaygroundClient(300, 'AAPL', '2021-01-04', '2021-01-31', RepositorySource.POLYGON)
        # playground_client = BacktesterPlaygroundClient(300, 'AAPL', '2021-01-04', '2021-01-31', RepositorySource.CSV, filename='training_data.csv')
        
        print('playground_id: ', playground_client.id)
        
        result = playground_client.place_order('AAPL', 10, OrderSide.SELL_SHORT)
                
        playground_client.tick(6000)
        
        tick_delta = playground_client.flush_new_state_buffer()[0]
        
        print('tick_delta #1: ', tick_delta)
        
                
        
    except Exception as e:
        print('Exception found: ', e)
        raise(e)