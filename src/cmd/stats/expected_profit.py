import numpy as np
import scipy.integrate as integrate
import plotly.graph_objs as go
from fetch_options import fetch_options
import distributions
import sys
import json

# to run this script:
# python expected_profit.py /Users/jamal/projects/slack-trading/src/cmd/stats/transform_data/supertrend_4h_1h_stoch_rsi_15m_up/candles-SPX-15/best_fit_percent_change-1440.json

# Import the distribution
input_file = sys.argv[1]
with open(input_file, 'r') as file:
    data = json.load(file)

# Get the distribution
print(f"using: {data}")

# Parameters for the beta distribution
lower_limit, upper_limit, distribution = distributions.dist(data)

percent_change_pdf = distribution.pdf

# Define the profit function for a call option based on percent change
def profit_function_call_percent(percent_change, stock_price, strike_price, premium):
    new_stock_price = stock_price * (1 + percent_change / 100)
    return max(new_stock_price - strike_price, 0) - premium

# Define the profit function for a call option based on percent change
def profit_function_put_percent(percent_change, stock_price, strike_price, premium):
    new_stock_price = stock_price * (1 + percent_change / 100)
    return max(strike_price - new_stock_price, 0) - premium


# Function to integrate: profit function * PDF
def call_integrand(percent_change):
    return profit_function_call_percent(percent_change, stock_price, strike_price, premium) * percent_change_pdf(percent_change)

def put_integrand(percent_change):
    return profit_function_put_percent(percent_change, stock_price, strike_price, premium) * percent_change_pdf(percent_change)

stock, options = fetch_options()

# Base stock price
stock_price = (stock.bid + stock.ask) / 2

print(f"Stock Price: {stock_price:.2f}")

for option in options:
    # Print the option
    print(option.description)

    # Parameters for the option
    strike_price = option.strike
    premium = option.ask

    # Integrate the expected profit over the range of percent changes
    if option.option_type == 'call':
        expected_profit, _ = integrate.quad(call_integrand, lower_limit, upper_limit)
    elif option.option_type == 'put':
        expected_profit, _ = integrate.quad(put_integrand, lower_limit, upper_limit)
    else:
        raise ValueError(f"Invalid option type: {option.option_type}")

    # Normalize by integrating the PDF over the same range
    # total_probability, _ = integrate.quad(percent_change_pdf, loc, loc + scale)

    # average_expected_profit = expected_profit / total_probability

    print(f"Expected Profit: {expected_profit:.2f}")




# # Generate x values for plotting the percent changes PDF
# x_values = np.linspace(loc, loc + scale, 1000)
# pdf_values = percent_change_pdf(x_values)

# # Create the Plotly figure
# fig = go.Figure()

# # Plot the Expected Profit Area
# fig.add_trace(go.Scatter(x=x_values, y=[integrand(x) for x in x_values], 
#                          fill='tozeroy', name='Expected Profit Area', line=dict(color='orange', width=0), fillcolor='rgba(255,165,0,0.3)'))

# # Update layout
# fig.update_layout(
#     title='Probability Density Function of Percent Changes with Expected Profit Area',
#     xaxis_title='Percent Change',
#     yaxis_title='Density',
#     legend_title='Legend',
#     template='plotly_white'
# )

# # Show the plot
# fig.show()
