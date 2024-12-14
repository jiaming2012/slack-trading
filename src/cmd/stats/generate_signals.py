import argparse
import datetime
import pandas as pd
import pandas_ta as ta
from utils import fetch_polygon_stock_chart_aggregated

parser = argparse.ArgumentParser(description='generate signals for a model.')
parser.add_argument('--start-date', type=str, help='The start date of the signals', required=True)
parser.add_argument('--end-date', type=str, help='The end date of the signals', required=True)
parser.add_argument('--symbol', type=str, help='The symbol to generate signals for', required=True)
parser.add_argument('--eventFn', type=str, help='The event function to use for generating signals', required=True)

args = parser.parse_args()
start_date = datetime.datetime.strptime(args.start_date, '%Y-%m-%d')
end_date = datetime.datetime.strptime(args.end_date, '%Y-%m-%d')

# Fetch htf data
htf_data = fetch_polygon_stock_chart_aggregated(args.symbol, 60, 'minute', start_date, end_date)
supertrend = ta.supertrend(htf_data['High'], htf_data['Low'], htf_data['Close'], length=50, multiplier=3)
htf_df = pd.concat([htf_data, supertrend], axis=1)

# Fetch ltf data
ltf_data = fetch_polygon_stock_chart_aggregated(args.symbol, 5, 'minute', start_date, end_date)
stochrsi = ta.stochrsi(ltf_data['Close'], rsi_length=14, stoch_length=14, k=3, d=3)
ltf_df = pd.concat([ltf_data, stochrsi], axis=1)

# Merge supertrend from htf_df into ltf_df
ltf_df = pd.merge_asof(ltf_df.sort_values('Date'), htf_df[['Date', 'SUPERT_50_3.0', 'SUPERTd_50_3.0']].sort_values('Date'), on='Date', direction='backward')

# Identify stochastic RSI crossovers
ltf_df['cross_below_80'] = (ltf_df['SUPERTd_50_3.0'] == -1) & (ltf_df['STOCHRSId_14_14_3_3'] > 80) & (ltf_df['STOCHRSIk_14_14_3_3'].shift(1) >= ltf_df['STOCHRSId_14_14_3_3'].shift(1)) & (ltf_df['STOCHRSIk_14_14_3_3'] < ltf_df['STOCHRSId_14_14_3_3'])
ltf_df['cross_above_20'] = (ltf_df['SUPERTd_50_3.0'] == 1) & (ltf_df['STOCHRSId_14_14_3_3'] < 20) & (ltf_df['STOCHRSIk_14_14_3_3'].shift(1) <= ltf_df['STOCHRSId_14_14_3_3'].shift(1)) & (ltf_df['STOCHRSIk_14_14_3_3'] > ltf_df['STOCHRSId_14_14_3_3'])

# Set display options to print all rows and columns
pd.set_option('display.max_rows', None)
# pd.set_option('display.max_columns', None)

print(ltf_df.tail(400))