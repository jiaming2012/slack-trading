import numpy as np
import pandas as pd
from scipy.stats import ks_2samp
import plotly.graph_objs as go

# Sample data: Stock prices (replace with actual stock price data)
np.random.seed(42)
stock_prices_1 = np.random.normal(100, 10, 1000)  # Stock prices for period 1
stock_prices_2 = np.random.normal(105, 15, 1000)  # Stock prices for period 2

# Perform the K-S test
ks_stat, p_value = ks_2samp(stock_prices_1, stock_prices_2)
print(f"KS Statistic: {ks_stat}, p-value: {p_value}")

# Plot the distributions using Plotly
fig = go.Figure()

# Add histogram for period 1
fig.add_trace(go.Histogram(
    x=stock_prices_1,
    histnorm='probability density',
    name='Stock Prices Period 1',
    opacity=0.75,
    marker=dict(color='blue')
))

# Add histogram for period 2
fig.add_trace(go.Histogram(
    x=stock_prices_2,
    histnorm='probability density',
    name='Stock Prices Period 2',
    opacity=0.75,
    marker=dict(color='red')
))

# Update layout
fig.update_layout(
    title='Comparison of Stock Price Distributions (K-S Test)',
    xaxis_title='Stock Price',
    yaxis_title='Density',
    barmode='overlay',
    legend=dict(x=0.8, y=1),
    annotations=[
        dict(
            text=f'KS Statistic: {ks_stat:.4f}<br>p-value: {p_value:.4f}',
            x=0.7,
            y=0.95,
            xref='paper',
            yref='paper',
            showarrow=False,
            font=dict(size=12)
        )
    ]
)

# Show the plot
fig.show()
