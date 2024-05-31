import numpy as np
import scipy.integrate as integrate
import plotly.graph_objs as go
from datetime import date
import pytz
import sys
import json
from fetch_options import fetch_options, generate_long_vertical_spreads, generate_short_vertical_spreads, filter_calls, filter_puts, Option, Spread, SpreadType
from market_info import time_to_option_contract_expiration_in_minutes
import distributions

# to run this script:
# python expected_profit.py /Users/jamal/projects/slack-trading/src/cmd/stats/transform_data/supertrend_4h_1h_stoch_rsi_15m_up/candles-SPX-15/best_fit_percent_change-1440.json

def expiration_in_days(time_until_expiration_in_minutes: int, today: date):
    nearest_contract_expiration = time_to_option_contract_expiration_in_minutes(today)

    if time_until_expiration_in_minutes <= nearest_contract_expiration:
        return 0
    
    days = time_until_expiration_in_minutes / 60 / 24
    return max(1, round(days))

def parse_option_expiration_in_days(fileURL: str, today: date):
    # parse 360 from /Users/jamal/projects/slack-trading/src/cmd/stats/clean_data_pdf/candles-SPX-15/best_fit_percent_change-360.json 
    parts = fileURL.split('-')
    minutes = int(parts[-1].split('.')[0])
    return expiration_in_days(minutes, today)

def parse_symbol(fileURL):
    # parse SPX from /Users/jamal/projects/slack-trading/src/cmd/stats/clean_data_pdf/candles-SPX-15/best_fit_percent_change-360.json 
    parts = fileURL.split('/')
    return parts[-2].split('-')[1]

def profit_function_call_spread(percent_change, stock_price, spread: Spread):
    long_call_profit = profit_function_long_call(percent_change, stock_price, spread.long_option.strike, spread.long_option.ask)
    short_call_profit = profit_function_short_call(percent_change, stock_price, spread.short_option.strike, spread.short_option.bid)
    return long_call_profit + short_call_profit

def profit_function_put_spread(percent_change, stock_price, spread: Spread):
    long_put_profit = profit_function_long_put(percent_change, stock_price, spread.long_option.strike, spread.long_option.ask)
    short_put_profit = profit_function_short_put(percent_change, stock_price, spread.short_option.strike, spread.short_option.bid)
    return long_put_profit + short_put_profit

def profit_function_long_put_spread(percent_change, stock_price, spread: Spread):
    long_put_profit = profit_function_long_put(percent_change, stock_price, spread.long_option.strike, spread.long_option.ask)
    short_put_profit = profit_function_short_put(percent_change, stock_price, spread.short_option.strike, spread.short_option.bid)
    return long_put_profit + short_put_profit

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
def generate_call_spread_integrand(stock_price: float, spread: Spread):
    def call_integrand(percent_change):
        return profit_function_call_spread(percent_change, stock_price, spread) * percent_change_pdf(percent_change)
    return call_integrand

def generate_put_spread_integrand(stock_price: float, spread: Spread):
    def put_integrand(percent_change):
        return profit_function_put_spread(percent_change, stock_price, spread) * percent_change_pdf(percent_change)
    return put_integrand


if __name__ == "__main__":
    inDir = sys.argv[1]
    with open(inDir, 'r') as file:
        data = json.load(file)

    # Get the distribution
    print(f"using: {data}")

    # Parameters for the distribution
    lower_limit, upper_limit, distribution = distributions.dist(data)

    percent_change_pdf = distribution.pdf

    today = date.today()
    symbol = parse_symbol(inDir)
    expirationInDays = parse_option_expiration_in_days(inDir, today)

    print(f"symbol: {symbol}, expirationInDays: {expirationInDays}")

    stock, options = fetch_options(symbol, expirationInDays, 10, 3)

    # Base stock price
    stock_price = (stock.bid + stock.ask) / 2

    print(f"Stock Price: {stock_price:.2f}")

    long_call_spreads_and_profits = []
    long_put_spreads_and_profits = []
    short_call_spreads_and_profits = []
    short_put_spreads_and_profits = []

    calls = filter_calls(options)
    
    long_call_spreads = generate_long_vertical_spreads(calls)

    for spread in long_call_spreads:
        if spread.type != SpreadType.VERTICAL_CALL:
            raise ValueError(f"Invalid option type: {spread.type}")

        # Integrate the expected profit over the range of percent changes
        long_call_spread_integrand = generate_call_spread_integrand(stock_price, spread)
        long_expected_profit, _ = integrate.quad(long_call_spread_integrand, lower_limit, upper_limit)
        debit_paid = spread.long_option.ask - spread.short_option.bid
        long_call_spreads_and_profits.append((spread, debit_paid, long_expected_profit))

    short_call_spreads = generate_short_vertical_spreads(calls)

    for spread in short_call_spreads:
        if spread.type != SpreadType.VERTICAL_CALL:
            raise ValueError(f"Invalid option type: {spread.type}")

        # Integrate the expected profit over the range of percent changes
        short_call_spread_integrand = generate_call_spread_integrand(stock_price, spread)
        short_expected_profit, _ = integrate.quad(short_call_spread_integrand, lower_limit, upper_limit)
        credit_received = spread.short_option.bid - spread.long_option.ask
        short_call_spreads_and_profits.append((spread, credit_received, short_expected_profit))

    puts = filter_puts(options)

    long_put_spreads = generate_long_vertical_spreads(puts)

    for spread in long_put_spreads:
        if spread.type != SpreadType.VERTICAL_PUT:
            raise ValueError(f"Invalid option type: {spread.type}")

        # Integrate the expected profit over the range of percent changes
        long_put_spread_integrand = generate_put_spread_integrand(stock_price, spread)
        long_expected_profit, _ = integrate.quad(long_put_spread_integrand, lower_limit, upper_limit)
        debit_paid = spread.long_option.ask - spread.short_option.bid
        long_put_spreads_and_profits.append((spread, debit_paid, long_expected_profit))

    short_put_spreads = generate_short_vertical_spreads(puts)

    for spread in short_put_spreads:
        if spread.type != SpreadType.VERTICAL_PUT:
            raise ValueError(f"Invalid option type: {spread.type}")

        # Integrate the expected profit over the range of percent changes
        short_put_spread_integrand = generate_put_spread_integrand(stock_price, spread)
        short_expected_profit, _ = integrate.quad(short_put_spread_integrand, lower_limit, upper_limit)
        credit_received = spread.short_option.bid - spread.long_option.ask
        short_put_spreads_and_profits.append((spread, credit_received, short_expected_profit))


    # Sort the list by expected profit
    long_call_spreads_and_profits.sort(key=lambda x: x[2], reverse=True)
    long_put_spreads_and_profits.sort(key=lambda x: x[2], reverse=True)
    short_call_spreads_and_profits.sort(key=lambda x: x[2], reverse=True)
    short_put_spreads_and_profits.sort(key=lambda x: x[2], reverse=True)

    # Print the options and their expected profits
    print("[LONG Call Spreads]:")
    for spread, debit_paid, long_expected_profit in long_call_spreads_and_profits:
        print(f"{spread.description()} - Debit Paid: {debit_paid:.2f} - Expected Profit: {long_expected_profit:.2f}")

    print("[SHORT Call Spreads]:")
    for spread, credit_received, short_expected_profit in short_call_spreads_and_profits:
        print(f"{spread.description()} - Credit Received: {credit_received:.2f} - Expected Profit: {short_expected_profit:.2f}")

    print("[LONG Put Spreads]:")
    for spread, debit_paid, long_expected_profit in long_put_spreads_and_profits:
        print(f"{spread.description()} - Debit Paid: {debit_paid:.2f} - Expected Profit: {long_expected_profit:.2f}")

    print("[SHORT Put Spreads]:")
    for spread, credit_received, short_expected_profit in short_put_spreads_and_profits:
        print(f"{spread.description()} - Credit Received: {credit_received:.2f} - Expected Profit: {short_expected_profit:.2f}")

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
