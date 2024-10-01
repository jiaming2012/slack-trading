import requests
from enum import Enum

class OrderSide(Enum):
    BUY = 'buy'
    SELL = 'sell'
    SELL_SHORT = 'sell_short'
    BUY_TO_COVER = 'buy_to_cover'

class BacktesterPlaygroundClient:
    def __init__(self, balance: float, symbol: str, start_date: str, stop_date: str):
        self.base_url = 'http://localhost:8080'
        self.id = self.create_playground(balance, symbol, start_date, stop_date)
        
    def tick(self, seconds: int) -> object:
        response = requests.post(
            f'{self.base_url}/playground/{self.id}/tick?seconds={seconds}'
        )
        
        if response.status_code != 200:
            raise Exception(response.text)
        
        return response.json()
        
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
    
    def create_playground(self, balance: float, symbol: str, start_date: str, stop_date: str) -> str:
        # Create a new playground
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
                    }
                }
            }
        )

        if response.status_code != 200:
            raise Exception(response.text)

        return response.json()['playground_id']

if __name__ == '__main__':
    try:
        playground_client = BacktesterPlaygroundClient(1000, 'AAPL', '2021-01-04', '2021-01-31')
        
        print('playground_id: ', playground_client.id)
        
        result = playground_client.place_order('AAPL', 10, OrderSide.BUY)
        
        print(result)
        
        result = playground_client.tick(60)
        
        print(result)
        
    except Exception as e:
        print(e)