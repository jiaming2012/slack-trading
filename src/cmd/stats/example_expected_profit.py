import numpy as np
import scipy.integrate as integrate
import plotly.graph_objs as go
from scipy.stats import norm

# Parameters for the normal distribution
mean = 100
std_dev = 10
num_samples = 1000

# Simulate stock prices
stock_prices = np.random.normal(loc=mean, scale=std_dev, size=num_samples)

# Define the PDF using the normal distribution
pdf = norm(loc=mean, scale=std_dev).pdf
x_values = np.linspace(mean - 3*std_dev, mean + 3*std_dev, 1000)
pdf_values = pdf(x_values)

# Define the profit function for a call option
def profit_function(stock_price, strike_price, premium):
    return max(stock_price - strike_price, 0) - premium

# Parameters for the option
strike_price = 105.0
premium = 1.1

# Function to integrate: profit function * PDF
def integrand(stock_price):
    return profit_function(stock_price, strike_price, premium) * pdf(stock_price)

# Integrate the expected profit
expected_profit, _ = integrate.quad(integrand, mean - 3*std_dev, mean + 3*std_dev)

print(f"Expected Profit: {expected_profit:.2f}")

# Create the Plotly figure
fig = go.Figure()

# Plot the PDF
fig.add_trace(go.Scatter(x=x_values, y=pdf_values, mode='lines', name='PDF', line=dict(color='blue')))

# Plot the Expected Profit Area
fig.add_trace(go.Scatter(x=x_values, y=[integrand(x) / pdf(x) for x in x_values], 
                         fill='tozeroy', name='Expected Profit Area', line=dict(color='orange', width=0), fillcolor='rgba(255,165,0,0.3)'))

# Add a vertical line for the strike price
fig.add_trace(go.Scatter(x=[strike_price, strike_price], y=[0, max(pdf_values)], mode='lines', 
                         name='Strike Price', line=dict(color='red', dash='dash')))

# Update layout
fig.update_layout(
    title='Probability Density Function of Stock Prices with Expected Profit Area',
    xaxis_title='Stock Price',
    yaxis_title='Density',
    legend_title='Legend',
    template='plotly_white'
)

# Show the plot
fig.show()
