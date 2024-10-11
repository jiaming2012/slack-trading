import requests
from enum import Enum
from datetime import datetime, timedelta
import numpy as np
from urllib.parse import urlencode
from dataclasses import dataclass
from typing import List, Dict

try:
    from zoneinfo import ZoneInfo
except ImportError:
    from backports.zoneinfo import ZoneInfo

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
    quantity: int
    cost_basis: float
    pl: float
    
@dataclass
class Account:
    balance: float
    positions: Dict[str, Position]
    
    @property
    def pl(self) -> float:
        return sum([position.pl for position in self.positions.values()])
    
    @property
    def get_position(self, symbol) -> Position:
        return self.positions.get(symbol)
    
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
    def __init__(self, balance: float, symbol: str, start_date: str, stop_date: str, source: RepositorySource, filename: str = None):
        self.symbol = symbol
        self.base_url = 'http://localhost:8080'
        
        if source == RepositorySource.CSV:
            self.id = self.create_playground_csv(balance, symbol, start_date, stop_date, filename)
        elif source == RepositorySource.POLYGON:
            self.id = self.create_playground_polygon(balance, symbol, start_date, stop_date)
        else:
            raise Exception('Invalid source')

        self.account = self.fetch_account_state()
        self.current_candle = None
        self._is_backtest_complete = False
        
    def fetch_account_state(self) -> Account:
        response = requests.get(f'{self.base_url}/playground/{self.id}/account')
        
        if response.status_code != 200:
            raise Exception(response.text)
        
        obj = response.json()
        
        return Account(
            balance=obj['balance'],
            positions={
                symbol: Position(
                    symbol=symbol,
                    quantity=position['quantity'],
                    cost_basis=position['cost_basis'],
                    pl=position['pl']
                ) for symbol, position in obj['positions'].items()
            }
        )
    
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
        new_trades = current_state.get('new_trades')
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
        
        query_params = urlencode({
            'symbol': self.symbol,
            'from': fromStr,
            'to': toStr
        })
                
        response = requests.get(
            f'{self.base_url}/playground/{self.id}/candles?{query_params}'
        )
        
        if response.status_code != 200:
            raise Exception(response.text)
        
        resp = response.json()
        
        candles_data = resp.get('candles')
        if not candles_data:
            return []
        
        candles = [
            Candle(
                open=candle['open'],
                high=candle['high'],
                low=candle['low'],
                close=candle['close'],
                volume=candle['volume'],
                datetime=candle['datetime']
            ) for candle in candles_data
        ]
        
        return candles
        
    def tick(self, seconds: int) -> object:
        response = requests.post(
            f'{self.base_url}/playground/{self.id}/tick?seconds={seconds}'
        )
        
        if response.status_code != 200:
            raise Exception(response.text)
        
        new_state = response.json()
        
        new_candles = new_state.get('new_candles')
        if new_candles and len(new_candles) > 0:
            for candle in new_candles:
                if candle['symbol'] == self.symbol:
                    self.current_candle = candle['candle']
                    break
                
        self._is_backtest_complete = new_state['is_backtest_complete']
        
        self.account = self.fetch_account_state()
                
        return new_state
        
    def place_order(self, symbol: str, quantity: int, side: OrderSide) -> object:
        response = requests.post(
            f'{self.base_url}/playground/{self.id}/order',
            json={
                'symbol': symbol,
                'class': 'equity',
                'quantity': quantity,
                'side': side.value,
                'type': 'market',
                'duration': 'day'
            }
        )
        
        if response.status_code != 200:
            raise Exception(response.text)
        
        return response.json()
    
    def create_playground_csv(self, balance: float, symbol: str, start_date: str, stop_date: str, filename: str) -> str:
        response = requests.post(
            f'{self.base_url}/playground',
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
    
    def create_playground_polygon(self, balance: float, symbol: str, start_date: str, stop_date: str) -> str:
        response = requests.post(
            f'{self.base_url}/playground',
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
                        'type': 'polygon'
                    }
                }
            }
        )

        if response.status_code != 200:
            raise Exception(response.text)

        return response.json()['playground_id']

if __name__ == '__main__':
    try:
        playground_client = BacktesterPlaygroundClient(1000, 'AAPL', '2021-01-04', '2021-01-31', RepositorySource.POLYGON)
        
        print('playground_id: ', playground_client.id)
        
        result = playground_client.place_order('AAPL', 10, OrderSide.BUY)
        
        print(result)
        
        new_state = playground_client.tick(60)
        
        print(new_state)
        
        account = playground_client.fetch_account_state()
        
        print(account['positions']['AAPL']['pl'])
        
        print(account['balance'])
        
        print(playground_client.is_backtest_complete())
        
        candles = playground_client.fetch_candles(datetime(2021, 1, 4, 10, 40, 0, 0, ZoneInfo('US/Eastern')), datetime(2021, 1, 4, 11, 0, 0, 0, ZoneInfo('US/Eastern')))
        
        reward = playground_client.fetch_reward_from_new_trades(new_state, 2, 2)
        
        print('reward: ', reward)
        
        
    except Exception as e:
        print(e)