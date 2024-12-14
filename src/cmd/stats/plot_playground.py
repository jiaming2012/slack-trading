import argparse
import requests
import logging
import pandas as pd
import os
import datetime
import pytz
import plotly.graph_objects as go
from plotly.subplots import make_subplots
from utils import fetch_polygon_stock_chart_aggregated

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def fetch_playground(id: str, host: str) -> dict:
    url = f"{host}/playground/{id}/account"
    response = requests.get(url)
    response.raise_for_status()
    return response.json()

def get_trades(playground: dict) -> list:
    trades = []
    for order in playground['orders']:
        trades.extend(order['trades'])
            
    return trades

def get_meta(account: dict) -> dict:
    return account['meta']

def get_polygon_date(date_in_rtf_3339: str) -> datetime:
    date_stamp = date_in_rtf_3339[:date_in_rtf_3339.find('T')]
    return datetime.datetime.strptime(date_stamp, '%Y-%m-%d')

parser = argparse.ArgumentParser(description="The script will plot trades found in a playground.")
parser.add_argument('--playground-id', type=str, help='The id of the playground to plot', required=True)
parser.add_argument('--host', type=str, help='The host of the playground', default='http://localhost:8080')
args = parser.parse_args()

# Fetch playground
playground = fetch_playground(args.playground_id, args.host)

# Get trades from playground
trades = get_trades(playground)
    
buy_trades = [t for t in trades if t['quantity'] > 0]
buy_trades_df = pd.DataFrame(buy_trades)

sell_trades = [t for t in trades if t['quantity'] < 0]
sell_trades_df = pd.DataFrame(sell_trades)

meta = get_meta(playground)

if len(meta['symbols']) > 1:
    raise ValueError('Only one symbol is supported')

# Polygon input parameters
symbol = meta['symbols'][0]
timeframe_value = 5
timeframe_unit = 'minute'
from_date = get_polygon_date(meta['start_date'])
to_date = get_polygon_date(meta['end_date'])

df = fetch_polygon_stock_chart_aggregated(symbol, timeframe_value, timeframe_unit, from_date, to_date)

# Filter out rows with NaN values in the relevant columns
df = df.dropna(subset=['Open', 'High', 'Low', 'Close'])
    
print(f'df: {df}')
print(f'Min Volume: {df["Volume"].min()}')
print(f'Max Volume: {df["Volume"].max()}')

# Create subplots
# fig = make_subplots(rows=1, cols=1, shared_xaxes=True,
#                     subplot_titles=(symbol,))

# Add candlestick chart
# fig.add_trace(go.Candlestick(
#     x=df['Date'],
#     open=df['Open'],
#     high=df['High'],
#     low=df['Low'],
#     close=df['Close'],
#     increasing_line_color='green',
#     decreasing_line_color='red',
#     name='Candle'
# ), row=1, col=1)

fig = make_subplots(rows=2, cols=1, shared_xaxes=True,
                        vertical_spacing=0.1,
                        subplot_titles=('Trades', 'Position'))


# Add trades
fig.add_trace(go.Candlestick(
    x=df['Date'],
    open=df['Open'],
    high=df['High'],
    low=df['Low'],
    close=df['Close']
))

fig.add_trace(go.Scatter(
    x=buy_trades_df['create_date'],
    y=buy_trades_df['price'],
    mode='markers',
    marker=dict(symbol='triangle-up', size=10, color='blue'),
    name='Buy Orders'
), row=1, col=1)

fig.add_trace(go.Scatter(
    x=sell_trades_df['create_date'],
    y=sell_trades_df['price'],
    mode='markers',
    marker=dict(symbol='triangle-down', size=10, color='red'),
    name='Sell Orders'
), row=1, col=1)

fig.update_layout(
    title=f'Playground ID {args.playground_id}',
    yaxis_title='Price',
    xaxis_title='Date',
    xaxis_rangeslider_visible=False,
    xaxis=dict(
        tickformat='%Y-%m-%d %H:%M',
        tickangle=45,
        tickmode='auto'
    ),
    yaxis=dict(
        autorange=True,
    )
)

# Add positions

# Record the position throughout time
position_arr = []
ts = []
position = 0
for t in trades:
    position += t['quantity']
    position_arr.append(position)
    ts.append(t['create_date'])
    
position_df = pd.DataFrame({
    'Date': ts,
    'Position': position_arr
})

fig.add_trace(go.Scatter(
    x=position_df['Date'],
    y=position_df['Position'],
    mode='lines',
    line=dict(dash='dot', color='black'),
    name='Position'
), row=2, col=1)

# fig.update_layout(
#     yaxis=dict(
#         autorange=False,
#         range=[0, 500]
#     )
# )

# Add volume bar chart
# fig.add_trace(go.Bar(
#     x=df['Date'],
#     y=df['Volume'],
#     name='Volume',
#     yaxis='y2'
# ), row=2, col=1)

# Update layout to include secondary y-axis
# fig.update_layout(
#     yaxis2=dict(
#         title='Volume',
#         overlaying='y',
#         side='right'
#     )
# )

# Update x-axis and y-axis to scale
# fig.update_xaxes(type='category', row=1, col=1)
# fig.update_yaxes(row=1, col=1, autorange=True)
# fig.update_yaxes(type='linear', row=2, col=1)

# Show the plot
fig.show()