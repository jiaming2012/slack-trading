import pandas as pd
import os
import sys
from plotly.subplots import make_subplots
import plotly.graph_objects as go
import argparse

# Parse command line arguments
parser = argparse.ArgumentParser(description='Plot stock price data')
parser.add_argument('file_path', type=str, help='Path to the CSV file containing stock price data')

args = parser.parse_args()
file_path = args.file_path

# Check if the file exists
if not os.path.exists(file_path):
    print(f'Error: File "{file_path}" not found')
    sys.exit(1)

# Load data
data = pd.read_csv(file_path)
data['timestamp'] = pd.to_datetime(data['timestamp'])  # Ensure 'timestamp' is datetime type

# Ensure High is always greater than Low, Open, and Close
data['high'] = data[['open', 'high', 'low', 'close']].max(axis=1)
data['low'] = data[['open', 'high', 'low', 'close']].min(axis=1)

start_date = data['timestamp'].min()
start_date_formatted = start_date.strftime('%Y-%m-%d')
end_date = data['timestamp'].max()
end_date_formatted = end_date.strftime('%Y-%m-%d')

# Create the candlestick plot using Plotly
# fig = px.line(data, x='timestamp', y='Close', title=f'Stock Price from {start_date_formatted} to {end_date_formatted}')
subplot1_title = f'Stock Price from {start_date_formatted} to {end_date_formatted}'
fig = make_subplots(rows=1, cols=1, shared_xaxes=True,
                        vertical_spacing=0.2,
                        subplot_titles=(subplot1_title,))

fig.add_trace(go.Candlestick(
        x=data['timestamp'],
        open=data['open'],
        high=data['high'],
        low=data['low'],
        close=data['close'],
        increasing_line_color='green',
        decreasing_line_color='red',
        name='Candle'
    ), row=1, col=1)

# Show the plot
fig.show()
