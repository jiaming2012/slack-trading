from backtesting import Backtest, Strategy
from backtesting.lib import crossover
from backtesting.test import SMA
from lib import FetchPolygonDataframe
import pandas as pd
import pandas_ta as ta

class Supertrend(Strategy):
    def init(self):
        # self.supertrend = self.data.df['SUPERT_50_3.0', 'SUPERTd_50_3.0']
        # self.st_direction
        pass
        
    def next(self):
        st_direction = self.data.df['SUPERTd_50_3.0'].iloc[-1]
        
        if st_direction == 1 and not self.position:
            self.buy()
        elif st_direction == -1 and self.position:
            self.sell()

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

df = FetchPolygonDataframe('COIN', '2024-01-01T00:00:00Z', '2024-10-02T00:00:00Z', 1, 'day')

# # Calculate hlcc4 (average of High, Low, and two times Close)
# df['hlcc4'] = (df['High'] + df['Low'] + 2 * df['Close']) / 4

# # Calculate the Supertrend indicator using hlcc4
# supertrend = ta.supertrend(high=df['hlcc4'], low=df['hlcc4'], close=df['hlcc4'], length=50, multiplier=3)

supertrend = ta.supertrend(length=50, multiplier=3)

# Add the Supertrend values to the dataframe
df = pd.concat([df, supertrend], axis=1)

# print(df.tail(50))

# bt = Backtest(data, SmaCross, commission=.002,
#               exclusive_orders=True)

bt = Backtest(df, Supertrend, commission=.002,
              exclusive_orders=True)

stats = bt.run()
# # stats = bt.optimize(n=2)
print(stats)
bt.plot()
