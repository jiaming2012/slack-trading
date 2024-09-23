from backtesting import Backtest, Strategy
from backtesting.lib import crossover
from backtesting.test import SMA
import requests
import pandas as pd

def fetch_polygon_data(symbol, start, end, multiplier, timespan):
    url = 'http://localhost:8080/data/polygon?symbol={}&from={}&to={}&multiplier={}&timespan={}'.format(symbol, start, end, multiplier, timespan)
    response = requests.get(url)
    
    data = response.json()
        
    if response.status_code != 200:
        raise Exception('Error fetching data from Polygon')
        
    # Convert the JSON response into a DataFrame
    df = pd.DataFrame(data)

    # Convert the 'Datetime' column to Pandas datetime, recognizing the RFC 3339 format
    df['Datetime'] = pd.to_datetime(df['Datetime'])
    
    # Set 'Datetime' as the index
    df.set_index('Datetime', inplace=True)
        
    return df

class SmaCross(Strategy):
    def init(self):
        price = self.data.Close
        self.ma1 = self.I(SMA, price, 10)
        self.ma2 = self.I(SMA, price, 20)

    def next(self):
        if crossover(self.ma1, self.ma2):
            self.buy()
        elif crossover(self.ma2, self.ma1):
            self.sell()

data = fetch_polygon_data('GOOG', '2021-01-01T00:00:00Z', '2022-01-02T00:00:00Z', 1, 'day')

bt = Backtest(data, SmaCross, commission=.002,
              exclusive_orders=True)

stats = bt.run()
# stats = bt.optimize(n=2)
print(stats)
bt.plot()
