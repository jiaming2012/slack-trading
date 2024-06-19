import numpy as np
import scipy.integrate as integrate
import plotly.graph_objs as go
from datetime import date
import pytz
import sys
import json
from enum import Enum
from fetch_options import fetch_options, generate_long_iron_condors, generate_short_iron_condors, filter_calls, filter_puts, Stock, Option, IronCondor, SpreadType
from market_info import time_to_option_contract_expiration_in_minutes
import distributions
import argparse
from pprint import pprint

class EnumEncoder(json.JSONEncoder):
    def default(self, obj):
        if isinstance(obj, Enum):
            return obj.value
        return super().default(obj)

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

# Define the profit function for an iron condor based on percent change
def profit_function_iron_condor(percent_change, stock_price, condor: IronCondor):
    long_call_profit = profit_function_long_call(percent_change, stock_price, condor.long_call.strike, condor.long_call.ask)
    short_call_profit = profit_function_short_call(percent_change, stock_price, condor.short_call.strike, condor.short_call.bid)
    long_put_profit = profit_function_long_put(percent_change, stock_price, condor.long_put.strike, condor.long_put.ask)
    short_put_profit = profit_function_short_put(percent_change, stock_price, condor.short_put.strike, condor.short_put.bid)
    return long_call_profit + short_call_profit + long_put_profit + short_put_profit

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
def generate_iron_condor_integrand(stock_price: float, condor: IronCondor):
    def iron_condor_integrand(percent_change):
        return profit_function_iron_condor(percent_change, stock_price, condor) * percent_change_pdf(percent_change)
    return iron_condor_integrand


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="This script requires an input directory to a json file containing the best fit distribution."
                                                 "Optionally, you can pass a json file path containing the stock and option prices, as an argument."
                                                 "It calculates an option condor ladder by fetching the options for the given symbol and expiration date, "
                                                 "and calculating the expected profit for each option condor.")
    
    # Add arguments
    parser.add_argument('--distributionInDir', type=str, required=True, help="Required. The input directory to a json file containing the best fit distribution")
    parser.add_argument('--optionPricesInDir', type=str, nargs='?', help="Optional. The input directory to a json file containing the stock and option prices")
    parser.add_argument('--longOnly', type=bool, default=False, help="Optional. Default is False. Only calculate long iron condors.")
    parser.add_argument('--shortOnly', type=bool, default=False, help="Optional. Default is False. Only calculate short iron condors.")
    parser.add_argument('--json-output', type=str, default=False, help="Optional. Default is False. Output the results in json format. Hides all other standard output.")

    # Parse the arguments
    args = parser.parse_args()

    if args.json_output and args.json_output.lower() == 'true':
        args.json_output = True
    else:
        args.json_output = False

    with open(args.distributionInDir, 'r') as file:
        data = json.load(file)

    # Get the distribution
    if not args.json_output:
        print("using data:")
        pprint(data)

    # Parameters for the distribution
    lower_limit, upper_limit, distribution = distributions.dist(data)   # Parameters for the distribution

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

        stock, options = fetch_options(url, symbol, expirationInDays, 20, 3)

    # Base stock price
    stock_price = (stock.bid + stock.ask) / 2

    if not args.json_output:
        print(f"Stock Price: {stock_price:.2f}")

    long_iron_condor_profits = []
    short_iron_condor_profits = []

    if not args.shortOnly:
        long_iron_condors = generate_long_iron_condors(options, symbol)
        len_long_iron_condors = len(long_iron_condors)

        i = 0
        for condor in long_iron_condors:
            if condor.type != SpreadType.IRON_CONDOR:
                raise ValueError(f"Invalid option type: {condor.type}")

            # Integrate the expected profit over the range of percent changes
            iron_condor_integrand = generate_iron_condor_integrand(stock_price, condor)
            long_expected_profit, _ = integrate.quad(iron_condor_integrand, lower_limit, upper_limit)
            debit_paid = condor.long_call.ask - condor.short_call.bid + condor.long_put.ask - condor.short_put.bid
            long_iron_condor_profits.append((condor, debit_paid, long_expected_profit))

            i += 1
            if i % 20 == 0:
                if not args.json_output:
                    print(f"processed {i} / {len_long_iron_condors} long iron condors")

    if not args.longOnly:
        short_iron_condors = generate_short_iron_condors(options, symbol)
        len_short_iron_condors = len(short_iron_condors)

        i = 0
        for condor in short_iron_condors:
            if condor.type != SpreadType.IRON_CONDOR:
                raise ValueError(f"Invalid option type: {condor.type}")

            # Integrate the expected profit over the range of percent changes
            iron_condor_integrand = generate_iron_condor_integrand(stock_price, condor)
            short_expected_profit, _ = integrate.quad(iron_condor_integrand, lower_limit, upper_limit)
            credit_received = condor.short_call.bid - condor.long_call.ask + condor.short_put.bid - condor.long_put.ask
            short_iron_condor_profits.append((condor, credit_received, short_expected_profit))

            i += 1
            if i % 20 == 0:
                if not args.json_output:
                    print(f"processed {i} / {len_short_iron_condors} short iron condors")

    # Sort the list by expected profit
    long_iron_condor_profits.sort(key=lambda x: x[2], reverse=True)
    short_iron_condor_profits.sort(key=lambda x: x[2], reverse=True)

    output = []

    if not args.shortOnly:
        # Print the options and their expected profits
        if not args.json_output:
            print("[LONG Iron Condors]:")

        for condor, debit_paid, long_expected_profit in long_iron_condor_profits:
            output.append({
                "description": condor.description(),
                "type": condor.type,
                "long_call_symbol": condor.long_call.symbol,
                "long_call_expiration": condor.long_call.expiration,
                "long_put_symbol": condor.long_put.symbol,
                "long_put_expiration": condor.long_put.expiration,
                "short_call_symbol": condor.short_call.symbol,
                "short_call_expiration": condor.short_call.expiration,
                "short_put_symbol": condor.short_put.symbol,
                "short_put_expiration": condor.short_put.expiration,
                "debit_paid": str(debit_paid),
                "expected_profit": str(long_expected_profit)
            })

            if not args.json_output:
                print(f"{condor.description()} - Debit Paid: {debit_paid:.2f} - Expected Profit: {long_expected_profit:.2f}")

    if not args.longOnly:
        if not args.json_output:
            if not args.json_output:
                print("[SHORT Iron Condors]:")

        for condor, credit_received, short_expected_profit in short_iron_condor_profits:
            output.append({
                "description": condor.description(),
                "type": condor.type,
                "long_call_symbol": condor.long_call.symbol,
                "long_call_expiration": condor.long_call.expiration,
                "long_put_symbol": condor.long_put.symbol,
                "long_put_expiration": condor.long_put.expiration,
                "short_call_symbol": condor.short_call.symbol,
                "short_call_expiration": condor.short_call.expiration,
                "short_put_symbol": condor.short_put.symbol,
                "short_put_expiration": condor.short_put.expiration,
                "credit_received": str(credit_received),
                "expected_profit": str(short_expected_profit)
            })

            if not args.json_output:
                print(f"{condor.description()} - Credit Received: {credit_received:.2f} - Expected Profit: {short_expected_profit:.2f}")

    if args.json_output:
        print(json.dumps(output, cls=EnumEncoder))