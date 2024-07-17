import numpy as np
import plotly.graph_objs as go
from datetime import date
import pytz
import sys
import json
from enum import Enum
from fetch_options import fetch_options, generate_long_vertical_spreads, generate_short_vertical_spreads, filter_calls, filter_puts, Stock, Option, Spread, SpreadType
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
    parts = fileURL.split('-')
    minutes = int(parts[-1].split('.')[0])
    return expiration_in_days(minutes, today)

def parse_symbol(fileURL):
    parts = fileURL.split('/')
    return parts[-1].split('-')[2]

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

def profit_function_long_call(percent_change, stock_price, strike_price, premium):
    new_stock_price = stock_price * (1 + percent_change / 100)
    return max(new_stock_price - strike_price, 0) - premium

def profit_function_short_call(percent_change, stock_price, strike_price, premium):
    new_stock_price = stock_price * (1 + percent_change / 100)
    return premium - max(new_stock_price - strike_price, 0)

def profit_function_long_put(percent_change, stock_price, strike_price, premium):
    new_stock_price = stock_price * (1 + percent_change / 100)
    return max(strike_price - new_stock_price, 0) - premium

def profit_function_short_put(percent_change, stock_price, strike_price, premium):
    new_stock_price = stock_price * (1 + percent_change / 100)
    return premium - max(strike_price - new_stock_price, 0)

def approximate_expected_profit(stock_price, spread, lower_limit, upper_limit, steps=200):
    percent_changes = np.linspace(lower_limit, upper_limit, steps)
    profits = np.array([profit_function_call_spread(p, stock_price, spread) for p in percent_changes])
    pdf_values = np.array([percent_change_pdf(p) for p in percent_changes])

    # Check for nan or inf values and handle them
    profits = np.nan_to_num(profits, nan=0.0, posinf=0.0, neginf=0.0)
    pdf_values = np.nan_to_num(pdf_values, nan=0.0, posinf=0.0, neginf=0.0)

    expected_profit = np.sum(profits * pdf_values) * (upper_limit - lower_limit) / steps
    return expected_profit

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="This script requires an input directory to a json file containing the best fit distribution."
                                                 "Optionally, you can pass a json file path containing the stock and option prices, as an argument."
                                                 "It calculates an option spread ladder by fetching the options for the given symbol and expiration date, "
                                                 "and calculating the expected profit for each option spread.")
    
    parser.add_argument('--distributionInDir', type=str, required=True, help="Required. The input directory to a json file containing the best fit distribution")
    parser.add_argument('--optionPricesInDir', type=str, nargs='?', help="Optional. The input directory to a json file containing the stock and option prices")
    parser.add_argument('--json-output', type=str, default=False, help="Optional. Default is False. Output the results in json format. Hides all other standard output.")

    args = parser.parse_args()

    if args.json_output.lower() == 'true':
        args.json_output = True
    else:
        args.json_output = False

    with open(args.distributionInDir, 'r') as file:
        data = json.load(file)

    if not args.json_output:
        print("using data:")
        pprint(data)

    lower_limit, upper_limit, distribution = distributions.dist(data)
    if lower_limit == -np.inf:
        lower_limit = -99

    if upper_limit == np.inf:
        upper_limit = 99

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

    stock_price = (stock.bid + stock.ask) / 2

    if not args.json_output:
        print(f"Stock Price: {stock_price:.2f}")

    long_call_spreads_and_profits = []
    long_put_spreads_and_profits = []
    short_call_spreads_and_profits = []
    short_put_spreads_and_profits = []

    calls = filter_calls(options)
    # long_call_spreads = generate_long_vertical_spreads(calls, symbol)

    # for spread in long_call_spreads:
    #     if spread.type != SpreadType.VERTICAL_CALL:
    #         raise ValueError(f"Invalid option type: {spread.type}")

    #     long_expected_profit = approximate_expected_profit(stock_price, spread, lower_limit, upper_limit)
    #     debit_paid = spread.long_option.ask - spread.short_option.bid
    #     long_call_spreads_and_profits.append((spread, debit_paid, long_expected_profit))

    short_call_spreads = generate_short_vertical_spreads(calls, symbol)

    for spread in short_call_spreads:
        if spread.type != SpreadType.VERTICAL_CALL:
            raise ValueError(f"Invalid option type: {spread.type}")

        short_expected_profit = approximate_expected_profit(stock_price, spread, lower_limit, upper_limit)
        credit_received = spread.short_option.bid - spread.long_option.ask
        short_call_spreads_and_profits.append((spread, credit_received, short_expected_profit))    

    puts = filter_puts(options)
    # long_put_spreads = generate_long_vertical_spreads(puts, symbol)

    # for spread in long_put_spreads:
    #     if spread.type != SpreadType.VERTICAL_PUT:
    #         raise ValueError(f"Invalid option type: {spread.type}")

    #     long_expected_profit = approximate_expected_profit(stock_price, spread, lower_limit, upper_limit)
    #     debit_paid = spread.long_option.ask - spread.short_option.bid
    #     long_put_spreads_and_profits.append((spread, debit_paid, long_expected_profit))

    short_put_spreads = generate_short_vertical_spreads(puts, symbol)

    for spread in short_put_spreads:
        if spread.type != SpreadType.VERTICAL_PUT:
            raise ValueError(f"Invalid option type: {spread.type}")

        short_expected_profit = approximate_expected_profit(stock_price, spread, lower_limit, upper_limit)
        credit_received = spread.short_option.bid - spread.long_option.ask
        short_put_spreads_and_profits.append((spread, credit_received, short_expected_profit))


    long_call_spreads_and_profits.sort(key=lambda x: x[2], reverse=True)
    long_put_spreads_and_profits.sort(key=lambda x: x[2], reverse=True)
    short_call_spreads_and_profits.sort(key=lambda x: x[2], reverse=True)
    short_put_spreads_and_profits.sort(key=lambda x: x[2], reverse=True)

    output = []

    if not args.json_output:
        print("[LONG Call Spreads]:")

    for spread, debit_paid, long_expected_profit in long_call_spreads_and_profits:
        output.append({
            "description": spread.description(),
            "type": spread.type,
            "long_option_symbol": spread.long_option.symbol,
            "long_option_expiration": spread.long_option.expiration,
            "short_option_symbol": spread.short_option.symbol,
            "short_option_expiration": spread.short_option.expiration,
            "debit_paid": str(debit_paid),
            "expected_profit": str(long_expected_profit)
        })

        if not args.json_output:
            print(f"{spread.description()} - Debit Paid: {debit_paid:.2f} - Expected Profit: {long_expected_profit:.2f}")

    if not args.json_output:
        print("[SHORT Call Spreads]:")

    for spread, credit_received, short_expected_profit in short_call_spreads_and_profits:
        output.append({
            "description": spread.description(),
            "type": spread.type,
            "long_option_symbol": spread.long_option.symbol,
            "long_option_expiration": spread.long_option.expiration,
            "short_option_symbol": spread.short_option.symbol,
            "short_option_expiration": spread.short_option.expiration,
            "credit_received": str(credit_received),
            "expected_profit": str(short_expected_profit)
        })

        if not args.json_output:
            print(f"{spread.description()} - Credit Received: {credit_received:.2f} - Expected Profit: {short_expected_profit:.2f}")

    if not args.json_output:
        print("[LONG Put Spreads]:")
    
    for spread, debit_paid, long_expected_profit in long_put_spreads_and_profits:
        output.append({
            "description": spread.description(),
            "type": spread.type,
            "long_option_symbol": spread.long_option.symbol,
            "long_option_expiration": spread.long_option.expiration,
            "short_option_symbol": spread.short_option.symbol,
            "short_option_expiration": spread.short_option.expiration,
            "debit_paid": str(debit_paid),
            "expected_profit": str(long_expected_profit)
        })

        if not args.json_output:
            print(f"{spread.description()} - Debit Paid: {debit_paid:.2f} - Expected Profit: {long_expected_profit:.2f}")

    if not args.json_output:
        print("[SHORT Put Spreads]:")
    
    for spread, credit_received, short_expected_profit in short_put_spreads_and_profits:
        output.append({
            "description": spread.description(),
            "type": spread.type,
            "long_option_symbol": spread.long_option.symbol,
            "long_option_expiration": spread.long_option.expiration,
            "short_option_symbol": spread.short_option.symbol,
            "short_option_expiration": spread.short_option.expiration,
            "credit_received": str(credit_received),
            "expected_profit": str(short_expected_profit)
        })

        if not args.json_output:
            print(f"{spread.description()} - Credit Received: {credit_received:.2f} - Expected Profit: {short_expected_profit:.2f}")

    if args.json_output:
        print(json.dumps(output, cls=EnumEncoder))
