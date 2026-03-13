import numpy as np
from scipy.stats import anderson_ksamp
import plotly.graph_objs as go

# Sample data: Stock prices (replace with actual stock price data)
np.random.seed(42)
stock_prices_1 = np.random.normal(105, 10, 1000)  # Stock prices for period 1
stock_prices_2 = np.random.normal(100, 10, 1000)  # Stock prices for period 2

# Perform the Anderson-Darling test
ad_stat, critical_values, p_value = anderson_ksamp([stock_prices_1, stock_prices_2])

# If the p_value is less than the confidence level (e.g., 0.05), reject the null hypothesis that the samples are drawn from the same distribution
significance_level = 0.05
print(f"AD Statistic: {ad_stat}, p-value: {p_value}")

if p_value < significance_level:
    print("Reject the null hypothesis that the samples are drawn from the same distribution")
else:
    print("Fail to reject the null hypothesis that the samples are drawn from the same distribution")

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
    title='Comparison of Stock Price Distributions (A-D Test)',
    xaxis_title='Stock Price',
    yaxis_title='Density',
    barmode='overlay',
    legend=dict(x=0.8, y=1),
    annotations=[
        dict(
            text=f'AD Statistic: {ad_stat:.4f}<br>p-value: {p_value:.4f}',
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
