from loguru import logger
from src.cmd.stats.playground_types import RepositorySource, OrderSide, LiveAccountType
from rpc.playground_pb2 import CreatePolygonPlaygroundRequest, DeletePlaygroundRequest, GetAccountRequest, GetCandlesRequest, NextTickRequest, PlaceOrderRequest, TickDelta, GetOpenOrdersRequest, Order, AccountMeta, Bar, CreateLivePlaygroundRequest, Repository, Candle as pb_Candle
from backtester_playground_client_grpc import BacktesterPlaygroundClient, OrderSide, RepositorySource, PlaygroundEnvironment, Repository, CreatePolygonPlaygroundRequest, InvalidParametersException, PlaceOrderSideNotAllowedException
import datetime
from typing import List, Dict
import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
import scipy.stats as stats
import seaborn as sns

def bars_to_dataframe(bars: List[Bar]) -> pd.DataFrame:
    records = []
    for bar in bars:
        record = {
            "datetime": pd.to_datetime(bar.datetime),  # convert string to datetime64
            "volume": bar.volume,
            "open": bar.open,
            "close": bar.close,
            "high": bar.high,
            "low": bar.low,
            "superT_50_3": bar.superT_50_3,
            "superD_50_3": bar.superD_50_3,
            "superL_50_3": bar.superL_50_3,
            "superS_50_3": bar.superS_50_3,
            "stochrsi_k_14_14_3_3": bar.stochrsi_k_14_14_3_3,
            "stochrsi_d_14_14_3_3": bar.stochrsi_d_14_14_3_3,
            "atr_14": bar.atr_14,
            "sma_50": bar.sma_50,
            "sma_100": bar.sma_100,
            "sma_200": bar.sma_200,
            "stochrsi_cross_above_20": bar.stochrsi_cross_above_20,
            "stochrsi_cross_below_80": bar.stochrsi_cross_below_80,
            "close_lag_1": bar.close_lag_1,
            "close_lag_2": bar.close_lag_2,
            "close_lag_3": bar.close_lag_3,
            "close_lag_4": bar.close_lag_4,
            "close_lag_5": bar.close_lag_5,
            "close_lag_6": bar.close_lag_6,
            "close_lag_7": bar.close_lag_7,
            "close_lag_8": bar.close_lag_8,
            "close_lag_9": bar.close_lag_9,
            "close_lag_10": bar.close_lag_10,
            "close_lag_11": bar.close_lag_11,
            "close_lag_12": bar.close_lag_12,
            "close_lag_13": bar.close_lag_13,
            "close_lag_14": bar.close_lag_14,
            "close_lag_15": bar.close_lag_15,
            "close_lag_16": bar.close_lag_16,
            "close_lag_17": bar.close_lag_17,
            "close_lag_18": bar.close_lag_18,
            "close_lag_19": bar.close_lag_19,
            "close_lag_20": bar.close_lag_20,
            "cdl_hammer": bar.cdl_hammer,
            "cdl_doji_10_0_1": bar.cdl_doji_10_0_1,
            "psar_long_value": bar.psar_long_value,
            "psar_short_value": bar.psar_short_value
        }
        records.append(record)
    
    df = pd.DataFrame(records)
    return df

# Add a console sink
logger.add(
    sink=lambda msg: print(msg, end=""),  # Print to console
    format="{time:YYYY-MM-DD HH:mm:ss} | {level} | {message}",
    level="INFO"
)

balance = 10000
symbol = 'COIN'
twirp_host = 'http://localhost:5051'
playground_env = 'simulator'
live_account_type = None
start_date = '2024-01-01'
stop_date = '2024-12-31'
playground_client_id = 'options_sandbox_coin_2024'
repository_source = RepositorySource.POLYGON
repo_timespan_unit = 'day'
repo_timespan_mutliplier = 1

repo = Repository(
    symbol=symbol,
    timespan_multiplier=1,
    timespan_unit='day',
    indicators=["supertrend", "atr"],
    history_in_days=365
)
    
req = CreatePolygonPlaygroundRequest(
    balance=balance,
    start_date=start_date,
    stop_date=stop_date,
    repositories=[repo],
    client_id=playground_client_id,
    environment=playground_env,
    tags=['test', 'playground']
)

playground = BacktesterPlaygroundClient(req, live_account_type, repository_source, logger, twirp_host=twirp_host)

print('Created playground with ID:', playground.id)

timestampFrom = datetime.datetime.strptime(start_date, '%Y-%m-%d')
timestampTo = datetime.datetime.strptime(stop_date, '%Y-%m-%d')
# timestampTo = datetime.timedelta(days=31) + timestampFrom
candles = playground.fetch_candles_v3(
    symbol=symbol,
    period_in_seconds=86400,
    timestampFrom=timestampFrom,
    timestampTo=timestampTo,
)

print(f'Fetched {len(candles)} candles for {symbol} from {timestampFrom} to {timestampTo}')

df = bars_to_dataframe(candles)

print(df.head())

# Test lognormal
prices = df['close']

# Compute log returns
df["log_return"] = np.log(df["close"] / df["close"].shift(1))

print("\nLog Returns:")
print(df["log_return"])

log_returns = df["log_return"].replace([np.inf, -np.inf], np.nan).dropna()

# Shapiro-Wilk
shapiro_stat, shapiro_p = stats.shapiro(log_returns)
print(f"Shapiro-Wilk Test: W={shapiro_stat:.4f}, p-value={shapiro_p:.4f}")

# Anderson-Darling
anderson_result = stats.anderson(log_returns, dist='norm')
print("\nAnderson-Darling Test:")
print(f"Statistic: {anderson_result.statistic:.4f}")
for i, (cv, sig) in enumerate(zip(anderson_result.critical_values, anderson_result.significance_level)):
    print(f"  At {sig}% significance level: Critical Value = {cv:.4f}")

# Kolmogorov-Smirnov (standardized to mean 0 and std 1)
ks_stat, ks_p = stats.kstest((log_returns - log_returns.mean()) / log_returns.std(), 'norm')
print(f"\nKolmogorov-Smirnov Test: D={ks_stat:.4f}, p-value={ks_p:.4f}")

# Plotting the distribution
sns.histplot(df["close"], kde=True, bins=50)
plt.title("Distribution of Close Prices")
plt.show()

# Plotting the distribution
sns.histplot(log_returns, kde=True, bins=50)
plt.title("Distribution of Log Returns")
plt.show()

stats.probplot(log_returns, dist="norm", plot=plt)
plt.title("Q-Q Plot")
plt.show()

## heatmap of volatility by time and price bin
# df["price_bin"] = pd.cut(df["close"], bins=10)
# df["time_bin"] = pd.qcut(df.index, q=10)

# pivot = df.pivot_table(index="time_bin", columns="price_bin", values="log_return", aggfunc="std")
# sns.heatmap(pivot, cmap="coolwarm")
# plt.title("Volatility (Std Dev) by Time and Price Bin")
# plt.show()

df["bucket"] = pd.qcut(df["close"], q=5)
bucket_stats = df.groupby("bucket")["log_return"].agg(["mean", "std"])
bucket_stats.plot(kind="bar", title="Mean and Std per Close Price Bucket")
print(bucket_stats)

plt.figure(figsize=(10, 6))
sns.boxplot(x="bucket", y="log_return", data=df)

x_ticks = range(len(bucket_stats))
plt.bar(x_ticks, bucket_stats["mean"], width=0.3, label="mean", color="dodgerblue", alpha=0.5)
plt.bar(x_ticks, bucket_stats["std"], width=0.3, label="std", color="orange", alpha=0.5, bottom=bucket_stats["mean"])

# Clean up x-axis
plt.xticks(rotation=0, ha="center")
plt.title("Distribution of Log Returns by Price Bucket")
plt.ylabel("log_return")
plt.xlabel("Price Bucket")
plt.legend()
plt.tight_layout()
plt.show()

mean_return = df['log_return'].mean()
std_return = df['log_return'].std()
trading_days = 252  # typical number of trading days in a year
historical_volatility = std_return * np.sqrt(trading_days)

print(f"Mean Log Return: {mean_return:.4f}")
print(f"Standard Deviation of Log Returns: {std_return:.4f}")
print(f"Historical Volatility (Annualized): {historical_volatility:.4f}")