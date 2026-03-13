import numpy as np
import scipy.integrate as integrate
import json
from datetime import date
from enum import Enum
from fetch_options import fetch_options, generate_long_vertical_spreads, generate_short_vertical_spreads, filter_calls, filter_puts, Stock, Option, Spread, SpreadType
from market_info import time_to_option_contract_expiration_in_minutes
import distributions
import argparse
from pprint import pprint
from concurrent.futures import ProcessPoolExecutor

class EnumEncoder(json.JSONEncoder):
    def default(self, obj):
        if isinstance(obj, Enum):
            return obj.value
        return super().default(obj)

def expiration_in_days(time_until_expiration_in_minutes, today):
    nearest_contract_expiration = time_to_option_contract_expiration_in_minutes(today)
    if time_until_expiration_in_minutes <= nearest_contract_expiration:
        return 0
    days = time_until_expiration_in_minutes / 60 / 24
    return max(1, round(days))

def parse_option_expiration_in_days(fileURL, today):
    parts = fileURL.split('-')
    minutes = int(parts[-1].split('.')[0])
    return expiration_in_days(minutes, today)

def parse_symbol(fileURL):
    parts = fileURL.split('/')
    return parts[-1].split('-')[2]

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

def generate_call_spread_integrand(stock_price, spread, percent_change_pdf):
    def call_integrand(percent_change):
        return (profit_function_long_call(percent_change, stock_price, spread.long_option.strike, spread.long_option.ask) +
                profit_function_short_call(percent_change, stock_price, spread.short_option.strike, spread.short_option.bid)) * percent_change_pdf(percent_change)
    return call_integrand

def generate_put_spread_integrand(stock_price, spread, percent_change_pdf):
    def put_integrand(percent_change):
        return (profit_function_long_put(percent_change, stock_price, spread.long_option.strike, spread.long_option.ask) +
                profit_function_short_put(percent_change, stock_price, spread.short_option.strike, spread.short_option.bid)) * percent_change_pdf(percent_change)
    return put_integrand

def calculate_spread_profit(stock_price, spread, integrand, lower_limit, upper_limit):
    expected_profit, _ = integrate.quad(integrand, lower_limit, upper_limit)
    if spread.type == SpreadType.VERTICAL_CALL:
        net_cost = spread.long_option.ask - spread.short_option.bid
    else:
        net_cost = spread.short_option.bid - spread.long_option.ask
    return spread, net_cost, expected_profit

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Calculate option spread ladder by fetching options for given symbol and expiration date, and calculating expected profit.")
    parser.add_argument('--distributionInDir', type=str, required=True, help="Input directory to a JSON file containing the best fit distribution")
    parser.add_argument('--optionPricesInDir', type=str, nargs='?', help="Input directory to a JSON file containing the stock and option prices")
    parser.add_argument('--json-output', type=str, default='false', help="Output the results in JSON format. Hides all other standard output.")
    args = parser.parse_args()

    args.json_output = args.json_output.lower() == 'true'

    with open(args.distributionInDir, 'r') as file:
        data = json.load(file)

    if not args.json_output:
        print("using data:")
        pprint(data)

    lower_limit, upper_limit, distribution = distributions.dist(data)
    percent_change_pdf = distribution.pdf

    today = date.today()
    symbol = parse_symbol(args.distributionInDir)
    expirationInDays = parse_option_expiration_in_days(args.distributionInDir, today)

    if not args.json_output:
        print(f"symbol: {symbol}, expirationInDays: {expirationInDays}")

    if args.optionPricesInDir:
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

    calls = filter_calls(options)
    puts = filter_puts(options)

    short_call_spreads = generate_short_vertical_spreads(calls, symbol)
    short_put_spreads = generate_short_vertical_spreads(puts, symbol)

    output = []

    with ProcessPoolExecutor() as executor:
        futures = []
        for spread in short_call_spreads:
            if spread.type != SpreadType.VERTICAL_CALL:
                raise ValueError(f"Invalid option type: {spread.type}")
            integrand = generate_call_spread_integrand(stock_price, spread, percent_change_pdf)
            futures.append(executor.submit(calculate_spread_profit, stock_price, spread, integrand, lower_limit, upper_limit))

        for spread in short_put_spreads:
            if spread.type != SpreadType.VERTICAL_PUT:
                raise ValueError(f"Invalid option type: {spread.type}")
            integrand = generate_put_spread_integrand(stock_price, spread, percent_change_pdf)
            futures.append(executor.submit(calculate_spread_profit, stock_price, spread, integrand, lower_limit, upper_limit))

        for future in futures:
            spread, net_cost, expected_profit = future.result()
            output.append({
                "description": spread.description(),
                "type": spread.type,
                "long_option_timestamp": spread.long_option.timestamp,
                "long_option_symbol": spread.long_option.symbol,
                "long_option_expiration": spread.long_option.expiration,
                "long_option_avg_fill_price": spread.long_option.avg_fill_price,
                "short_option_timestamp": spread.short_option.timestamp,
                "short_option_symbol": spread.short_option.symbol,
                "short_option_expiration": spread.short_option.expiration,
                "short_option_avg_fill_price": spread.short_option.avg_fill_price,
                "net_cost": str(net_cost),
                "expected_profit": str(expected_profit)
            })

            if not args.json_output:
                print(f"{spread.description()} - Net Cost: {net_cost:.2f} - Expected Profit: {expected_profit:.2f}")

    if args.json_output:
        print(json.dumps(output, cls=EnumEncoder))
