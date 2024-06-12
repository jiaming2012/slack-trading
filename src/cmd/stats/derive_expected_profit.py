import numpy as np
import scipy.integrate as integrate
import plotly.graph_objs as go
from datetime import date
import pytz
import sys
import json
from market_info import time_to_option_contract_expiration_in_minutes
from fetch_options import Stock, Option, fetch_options
import distributions
import argparse
from pprint import pprint

def expiration_in_days(time_until_expiration_in_minutes: int, today: date):
    nearest_contract_expiration = time_to_option_contract_expiration_in_minutes(today)

    if time_until_expiration_in_minutes <= nearest_contract_expiration:
        return 0
    
    days = time_until_expiration_in_minutes / 60 / 24
    return max(1, round(days))

def parse_option_expiration_in_days(fileURL: str, today: date):
    # parse 360 from /Users/jamal/projects/slack-trading/src/cmd/stats/clean_data_pdf/candles-SPX-15/best_fit_percent_change-360.json 
    # /Users/jamal/projects/slack-trading/src/cmd/stats/transform_data/supertrend_4h_1h_stoch_rsi_15m_up/distributions/percent_change-candles-SPX-15-from-20240102_093000-to-20240531_160000-lookahead-240.json
    parts = fileURL.split('-')
    minutes = int(parts[-1].split('.')[0])
    return expiration_in_days(minutes, today)

def parse_symbol(fileURL):
    # parse SPX from
    # /Users/jamal/projects/slack-trading/src/cmd/stats/transform_data/supertrend_4h_1h_stoch_rsi_15m_up/distributions/percent_change-candles-SPX-15-from-20240102_093000-to-20240531_160000-lookahead-240.json
    parts = fileURL.split('/')
    return parts[-1].split('-')[2]

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

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="This script requires an input directory to a json file containing the best fit distribution."
                                                 "Optionally, you can pass a json file path containing the stock and option prices, as an argument."
                                                 "It fetches the options for the given symbol and expiration date, and calculates the expected profit for each option.")
    
    # Add arguments
    parser.add_argument('--distributionInDir', type=str, required=True, help="Required. The input directory to a json file containing the best fit distribution")
    parser.add_argument('--optionPricesInDir', type=str, nargs='?', help="Optional. The input directory to a json file containing the stock and option prices")
    parser.add_argument('--json-output', type=str, default=False, help="Optional. Default is False. Output the results in json format. Hides all other standard output.")

    # Parse the arguments
    args = parser.parse_args()

    if args.json_output.lower() == 'true':
        args.json_output = True
    else:
        args.json_output = False

    with open(args.distributionInDir, 'r') as file:
        data = json.load(file)

    # Get the distribution
    if not args.json_output:
        print("using data:")
        pprint(data)

    lower_limit, upper_limit, distribution = distributions.dist(data)  # Parameters for the distribution

    percent_change_pdf = distribution.pdf

    today = date.today()
    symbol = parse_symbol(args.distributionInDir)
    expirationInDays = parse_option_expiration_in_days(args.distributionInDir, today)

    if not args.json_output:
        print(f"symbol: {symbol}, expirationInDays: {expirationInDays}")

    if not sys.stdin.isatty():
        data = json.load(sys.stdin)
        stock = Stock(**data['stock'])
        options = [Option(**option) for option in data['options']]
    elif args.optionPricesInDir:
        if not args.json_output:
            print(f"Loading options data from {args.optionPricesInDir}")

        with open(args.optionPricesInDir, 'r') as file:
            data = json.load(file)

        stock = Stock(**data['stock'])
        options = [Option(**option) for option in data['options']]
    else:
        url = 'http://localhost:8080/options'

        if not args.json_output:
            print(f"Fetching options data from {url} ...")

        stock, options = fetch_options(url, symbol, expirationInDays, 10, 5)

    # Base stock price
    stock_price = (stock.bid + stock.ask) / 2

    if not args.json_output:
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

    output = []

    # Print the options and their expected profits
    if not args.json_output:
        print("[LONG Calls]:")

    for option, debit_paid, long_expected_profit in long_calls_options_and_profits:
        output.append({
            "description": option.description,
            "debit_paid": str(debit_paid),
            "expected_profit": str(long_expected_profit)
        })

        if not args.json_output:
            print(f"{option.description} - debit paid: {debit_paid:.2f} - Expected Profit: {long_expected_profit:.2f}")

    if not args.json_output:
        print("[SHORT Calls]:")

    for option, credit_received, short_expected_profit in short_calls_options_and_profits:
        output.append({
            "description": option.description,
            "credit_received": str(credit_received),
            "expected_profit": str(short_expected_profit)
        })
        
        if not args.json_output:
            print(f"{option.description} - credit received: {credit_received:.2f} - Expected Profit: {short_expected_profit:.2f}")

    if not args.json_output:
        print("[LONG Puts]:")

    for option, debit_paid, long_expected_profit in long_puts_options_and_profits:
        output.append({
            "description": option.description,
            "debit_paid": str(debit_paid),
            "expected_profit": str(long_expected_profit)
        })

        if not args.json_output:
            print(f"{option.description} - debit paid: {debit_paid:.2f} - Expected Profit: {long_expected_profit:.2f}")

    if not args.json_output:
        print("[SHORT Puts]:")

    for option, credit_received, short_expected_profit in short_puts_options_and_profits:
        output.append({
            "description": option.description,
            "credit_received": str(credit_received),
            "expected_profit": str(short_expected_profit)
        })

        if not args.json_output:
            print(f"{option.description} - credit received: {credit_received:.2f} - Expected Profit: {short_expected_profit:.2f}")

    if args.json_output:
        print(json.dumps(output))

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
