import pandas as pd
import numpy as np
import plotly.express as px
import plotly.graph_objects as go
import sys
import os
import json
from scipy.stats import kstest, gaussian_kde, kurtosis as scipy_kurtosis
import distributions as dists
import argparse

def convert(o):
    if isinstance(o, np.generic):
        return o.item()
    elif np.isnan(o):
        return None
    elif np.isinf(o):
        return "Infinity"
    raise TypeError

# Create the parser
parser = argparse.ArgumentParser(description="This script requires an input directory to a csv file containing columns 'Time' and 'Percent_Change'. "
                                             "The script fits various distributions to the 'Percent_Change' data and performs the Kolmogorov-Smirnov test to find the best fit."
                                             "It then exports the best fit distribution and its parameters to a json file.")

# Add an argument
parser.add_argument('--inDir', type=str, required=True, help="The input directory to a csv file containing columns 'Time' and 'Percent_Change'")
parser.add_argument('--json-output', type=str, default=False, help="Optional. Default is False. Output the results in json format. Hides all other standard output.")

# Parse the arguments
args = parser.parse_args()

if args.json_output.lower() == 'true':
    args.json_output = True
else:
    args.json_output = False

if not args.json_output:
    # Now you can use args.inDir to get the value of the argument
    print(f"Loading data from {args.inDir}")

# Load data from Excel
df = pd.read_csv(args.inDir)
prices = df['Percent_Change'].dropna()

results = {}
for name, distribution in dists.distributions.items():
    try:
        # If the distribution is Erlang, round the shape parameter to the nearest integer
        # if name == 'Erlang':
        #     min_price = prices.min()
        #     if min_price < 0:
        #         prices = round(prices - min_price)
        #         transformation = {'type': 'shift', 'value': min_price}

        # Fit the distribution to the data
        params = distribution.fit(prices)

        # if name == 'Erlang':
        #     params = (round(params[0]),) + params[1:]
        
        # Perform the Kolmogorov-Smirnov test
        D, p_value = kstest(prices, distribution.name, args=params)
        
        # Store the results
        results[name] = {'params': params, 'D': D, 'p_value': p_value}

        # if transformation:
        #     results[name]['transformation'] = transformation

    except Exception as e:
        print(f"Could not fit {name} distribution: {e}")
        raise e

if not args.json_output:
    # Display the results
    for name, result in results.items():
        print(f"{name} Distribution: D={result['D']:.4f}, p-value={result['p_value']:.4f}")

# Plot the Q-Q plot for the best fitting distribution
best_fit = min(results, key=lambda k: results[k]['D'])
best_params = results[best_fit]['params']
dist = dists.distributions[best_fit]

if not args.json_output:
    print(f"The best fitting distribution is {best_fit} with parameters {best_params}")

distribution = dists.distributions[best_fit](*best_params)

# Calculate kurtosis for the left and right tails
random_variates = distribution.rvs(size=10000, random_state=0)

median = distribution.median()
left_tail = random_variates[random_variates < median]
right_tail = random_variates[random_variates > median]
left_kurtosis = scipy_kurtosis(left_tail, fisher=False)  # Set fisher=False to get Pearson's kurtosis
right_kurtosis = scipy_kurtosis(right_tail, fisher=False)

# Export the best fit distribution and its parameters to a json file
output = {
    'distribution': best_fit, 
    'params': best_params,
    'D': results[best_fit]['D'],
    'p_value': results[best_fit]['p_value'],
    'mean': convert(distribution.mean()),
    'median': convert(median),
    'variance': convert(distribution.var()),
    'std_dev': convert(np.sqrt(distribution.var())),
    # 'skewness': convert(distribution.stats(moments='s').tolist()),
    # 'kurtosis': convert(distribution.stats(moments='k').tolist()),
    'left_tail_kurtosis': left_kurtosis,
    'right_tail_kurtosis': right_kurtosis
}

# Calculate export directory
outPath = os.path.dirname(os.path.dirname(args.inDir)) # Go back two directories
outPath = os.path.join(outPath, 'distributions')
filename, _ = os.path.splitext(os.path.basename(args.inDir))
outDir = os.path.join(outPath, filename) + '.json'

with open(outDir, 'w') as f:
    json.dump(output, f)

if args.json_output:
    print(json.dumps({'outDir': outDir}))
else:
    print(f"Exporting results to {outDir}")

