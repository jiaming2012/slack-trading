from lib import FetchPolygonDataframe, Renko

df = FetchPolygonDataframe('COIN', '2024-01-01T00:00:00Z', '2024-10-02T00:00:00Z', 15, 'minute')
print(df.tail())
df = df.rename(columns={'Datetime': 'datetime', 'Close': 'close', 'High': 'high', 'Low': 'low', 'Open': 'open', 'Volume': 'volume'})

result = Renko(df, 10)

# mode can be one of: normal, nongap, wicks
renko_df = result.renko_df(mode='wicks')
print(type(renko_df))

# datetime represents the close of the Renko bar
print(renko_df.tail())

# rnk.plot()