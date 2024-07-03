import sys
import json
import pandas as pd
import plotly.graph_objects as go
from plotly.subplots import make_subplots
import argparse
import numpy as np
from typing import Dict, Any

def plot_candlestick(candle_data: Dict[str, Any], order_data: Dict[str, Any], option_data: Dict[str, Any], strike_price: float, timeframe: int) -> None:
    # Sample minute-level data
    df = candle_to_np(candle_data, timeframe)

    # Ensure High is always greater than Low, Open, and Close
    df['High'] = df[['Open', 'High', 'Low', 'Close']].max(axis=1)
    df['Low'] = df[['Open', 'High', 'Low', 'Close']].min(axis=1)

    # Sample order data (should match the time range of your candlestick data)
    df_orders = order_to_np(order_data)

    # Sample option data for XYZ (at 15-minute intervals)
    df_option = option_to_np(option_data)

    # Ensure High is always greater than Low, Open, and Close for options
    df_option['High'] = df_option[['Open', 'Close']].max(axis=1)
    df_option['Low'] = df_option[['Open', 'Close']].min(axis=1)

    # Define the strike price
    strike_price = 107

    # Create subplots
    fig = make_subplots(rows=2, cols=1, shared_xaxes=True,
                        vertical_spacing=0.2,
                        subplot_titles=(f'{timeframe}-Minute Candlestick Chart', 'Option Prices'))

    # Add candlestick chart
    fig.add_trace(go.Candlestick(
        x=df['Date'],
        open=df['Open'],
        high=df['High'],
        low=df['Low'],
        close=df['Close'],
        increasing_line_color='green',
        decreasing_line_color='red',
        name='Candle'
    ), row=1, col=1)

    # Add buy orders to candlestick chart
    buy_orders = df_orders[df_orders['Type'] == 'Buy']
    fig.add_trace(go.Scatter(
        x=buy_orders['Date'],
        y=buy_orders['Price'],
        mode='markers',
        marker=dict(symbol='triangle-up', size=10, color='blue'),
        name='Buy Orders'
    ), row=1, col=1)

    # Add sell orders to candlestick chart
    sell_orders = df_orders[df_orders['Type'] == 'Sell']
    fig.add_trace(go.Scatter(
        x=sell_orders['Date'],
        y=sell_orders['Price'],
        mode='markers',
        marker=dict(symbol='triangle-down', size=10, color='red'),
        name='Sell Orders'
    ), row=1, col=1)

    # Add dotted line connecting the buy and sell orders
    fig.add_trace(go.Scatter(
        x=df_orders['Date'],
        y=df_orders['Price'],
        mode='lines',
        line=dict(dash='dot', color='black'),
        name='Buy-Sell Line'
    ), row=1, col=1)

    # Add option close prices
    fig.add_trace(go.Scatter(
        x=df_option['Date'],
        y=df_option['Close'],
        mode='lines+markers',
        line=dict(color='purple'),
        name='Option Close Price'
    ), row=2, col=1)

    # Add buy orders to option chart
    fig.add_trace(go.Scatter(
        x=buy_orders['Date'],
        y=[df_option[df_option['Date'] == date]['Close'].values[0] for date in buy_orders['Date'] if not df_option[df_option['Date'] == date].empty],
        mode='markers',
        marker=dict(symbol='triangle-up', size=10, color='blue'),
        name='Buy Orders (Option)'
    ), row=2, col=1)

    # Add sell orders to option chart
    fig.add_trace(go.Scatter(
        x=sell_orders['Date'],
        y=[df_option[df_option['Date'] == date]['Close'].values[0] for date in sell_orders['Date'] if not df_option[df_option['Date'] == date].empty],
        mode='markers',
        marker=dict(symbol='triangle-down', size=10, color='red'),
        name='Sell Orders (Option)'
    ), row=2, col=1)

    # Add solid red strike price line to legend
    fig.add_trace(go.Scatter(
        x=[df['Date'].min(), df['Date'].max()],
        y=[strike_price, strike_price],
        mode='lines',
        line=dict(color="red", width=2),
        name='Strike Price'
    ), row=1, col=1)

    # Update layout for better visuals
    fig.update_layout(
        title=f'{timeframe}-Minute Candlestick Chart with Buy and Sell Orders, Option Prices, and Strike Price',
        yaxis_title='Price',
        xaxis2_title='Date',
        yaxis2_title='Option Price',
        xaxis_rangeslider_visible=False
    )

    # Show the plot
    fig.show()

def option_to_np(data: Dict[str, Any]) -> pd.DataFrame:
    df = pd.DataFrame()
    df['Date'] = pd.to_datetime(data['Date'])
    df['Open'] = np.array(data['Open'])
    df['Close'] = np.array(data['Close'])
    return df

def order_to_np(data: Dict[str, Any]) -> pd.DataFrame:
    df = pd.DataFrame()
    df['Date'] = pd.to_datetime(data['Date'])
    df['Price'] = np.array(data['Price'])
    df['Type'] = np.array(data['Type'])
    return df

def candle_to_np(data: Dict[str, Any], timeframeInMinutes: int) -> pd.DataFrame:    
    # Convert to numpy objects
    dates = pd.to_datetime(data['Date'])
    
    # Ensure the frequency is set to 'T'
    dates = pd.date_range(start=dates.min(), end=dates.max(), freq=f'{timeframeInMinutes}T')

    # Convert to numpy objects
    df = pd.DataFrame()
    df['Date'] = dates
    df['Open'] = np.array(data['Open'])
    df['High'] = np.array(data['High'])
    df['Low'] = np.array(data['Low'])
    df['Close'] = np.array(data['Close'])
    return df

def main() -> None:
    parser = argparse.ArgumentParser(description="Plot candlestick chart with buy/sell orders and option prices.")
    parser.add_argument('input', type=str, help='Input data in JSON format')
    args = parser.parse_args()

    # Read input data from standard input
    input_data = json.loads(args.input)
    
    candle_data = input_data['candle_data']
    order_data = input_data['order_data']
    option_data = input_data['option_data']
    strike_price = input_data['strike_price']

    plot_candlestick(candle_data, order_data, option_data, strike_price, 15)

if __name__ == "__main__":
    main()
