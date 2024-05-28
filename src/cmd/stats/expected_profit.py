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

# Define the profit function for a long call option based on percent change
def profit_function_long_call(percent_change, stock_price, strike_price, premium):
    new_stock_price = stock_price * (1 + percent_change / 100)
    return max(new_stock_price - strike_price, 0) - premium

# Define the profit function for a short call option based on percent change
def profit_function_short_call(percent_change, stock_price, strike_price, premium):
    new_stock_price = stock_price * (1 + percent_change / 100)
    return premium - max(new_stock_price - strike_price, 0)

# Define the profit function for a long put option based on percent change
def profit_function_long_put(percent_change, stock_price, strike_price, premium):
    new_stock_price = stock_price * (1 + percent_change / 100)
    return max(strike_price - new_stock_price, 0) - premium

# Define the profit function for a short put option based on percent change
def profit_function_short_put(percent_change, stock_price, strike_price, premium):
    new_stock_price = stock_price * (1 + percent_change / 100)
    return premium - max(strike_price - new_stock_price, 0)


# Function to integrate: profit function * PDF
def generate_long_call_integrand(stock_price, strike_price, premium):
    def call_integrand(percent_change):
        return profit_function_long_call(percent_change, stock_price, strike_price, premium) * percent_change_pdf(percent_change)
    return call_integrand


def generate_long_put_integrand(stock_price, strike_price, premium):
    def put_integrand(percent_change):
        return profit_function_long_put(percent_change, stock_price, strike_price, premium) * percent_change_pdf(percent_change)
    return put_integrand

def generate_short_call_integrand(stock_price, strike_price, premium):
    def call_integrand(percent_change):
        return profit_function_short_call(percent_change, stock_price, strike_price, premium) * percent_change_pdf(percent_change)
    return call_integrand

def generate_short_put_integrand(stock_price, strike_price, premium):
    def put_integrand(percent_change):
        return profit_function_short_put(percent_change, stock_price, strike_price, premium) * percent_change_pdf(percent_change)
    return put_integrand

symbol = data['symbol']
expirationInDays = data['expirationInDays']

print(f"symbol: {symbol}, expirationInDays: {expirationInDays}")

stock, options = fetch_options(symbol, expirationInDays)

# Base stock price
stock_price = (stock.bid + stock.ask) / 2

print(f"Stock Price: {stock_price:.2f}")

long_calls_options_and_profits = []
long_puts_options_and_profits = []
short_calls_options_and_profits = []
short_puts_options_and_profits = []

for option in options:
    # Parameters for the option
    strike_price = option.strike

    # Integrate the expected profit over the range of percent changes
    if option.option_type == 'call':
        long_call_integrand = generate_long_call_integrand(stock_price, strike_price, option.ask)
        long_expected_profit, _ = integrate.quad(long_call_integrand, lower_limit, upper_limit)
        long_calls_options_and_profits.append((option, option.ask, long_expected_profit))

        short_call_integrand = generate_short_call_integrand(stock_price, strike_price, option.bid)
        short_expected_profit, _ = integrate.quad(short_call_integrand, lower_limit, upper_limit)
        short_calls_options_and_profits.append((option, option.bid, short_expected_profit))
    elif option.option_type == 'put':
        long_put_integrand = generate_long_put_integrand(stock_price, strike_price, option.ask)
        long_expected_profit, _ = integrate.quad(long_put_integrand, lower_limit, upper_limit)
        long_puts_options_and_profits.append((option, option.ask, long_expected_profit))

        short_put_integrand = generate_short_put_integrand(stock_price, strike_price, option.bid)
        short_expected_profit, _ = integrate.quad(short_put_integrand, lower_limit, upper_limit)
        short_puts_options_and_profits.append((option, option.bid, short_expected_profit))
    else:
        raise ValueError(f"Invalid option type: {option.option_type}")

# Sort the list by expected profit
long_calls_options_and_profits.sort(key=lambda x: x[2], reverse=True)
long_puts_options_and_profits.sort(key=lambda x: x[2], reverse=True)
short_calls_options_and_profits.sort(key=lambda x: x[2], reverse=True)
short_puts_options_and_profits.sort(key=lambda x: x[2], reverse=True)

# Print the options and their expected profits
print("[LONG Calls]:")
for option, premium, long_expected_profit in long_calls_options_and_profits:
    print(f"{option.description} - debit paid: {premium:.2f} - Expected Profit: {long_expected_profit:.2f}")

print("[SHORT Calls]:")
for option, premium, short_expected_profit in short_calls_options_and_profits:
    print(f"{option.description} - credit received: {premium:.2f} - Expected Profit: {short_expected_profit:.2f}")

print("[LONG Puts]:")
for option, premium, long_expected_profit in long_puts_options_and_profits:
    print(f"{option.description} - debit paid: {premium:.2f} - Expected Profit: {long_expected_profit:.2f}")

print("[SHORT Puts]:")
for option, premium, short_expected_profit in short_puts_options_and_profits:
    print(f"{option.description} - credit received: {premium:.2f} - Expected Profit: {short_expected_profit:.2f}")

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
