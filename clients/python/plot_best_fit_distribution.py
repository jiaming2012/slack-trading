import pandas as pd
import numpy as np
import plotly.express as px
import plotly.graph_objects as go
import sys
from scipy.stats import kstest, gaussian_kde
import distributions as dists
import argparse

# Create the parser
parser = argparse.ArgumentParser(description="This script plots the best fitting distribution for the given data. "
                                             "It requires an input directory to a csv file containing columns 'Time' and 'Percent_Change'. "
                                             "The script fits various distributions to the 'Percent_Change' data and performs the Kolmogorov-Smirnov test to find the best fit.")

# Add an argument
parser.add_argument('--inDir', type=str, required=True, help="The input directory to a csv file containing columns 'Time' and 'Percent_Change'")

# Parse the arguments
args = parser.parse_args()

# Now you can use args.inDir to get the value of the argument
print(f"Loading data from {args.inDir}")

# Load data from Excel
df = pd.read_csv(args.inDir)
prices = df['Percent_Change'].dropna()

results = {}
for name, distribution in dists.distributions.items():
    try:
        # Fit the distribution to the data
        params = distribution.fit(prices)

        # If the distribution is Erlang, round the shape parameter to the nearest integer
        if name == 'Erlang':
            params = (round(params[0]),) + params[1:]
        
        # Perform the Kolmogorov-Smirnov test
        D, p_value = kstest(prices, distribution.name, args=params)
        
        # Store the results
        results[name] = {'params': params, 'D': D, 'p_value': p_value}
    except Exception as e:
        print(f"Could not fit {name} distribution: {e}")

# Plot the Q-Q plot for the best fitting distribution
best_fit = min(results, key=lambda k: results[k]['D'])
best_params = results[best_fit]['params']
dist = dists.distributions[best_fit]

# Q-Q plot
fig = go.Figure()
fig.add_trace(go.Scatter(x=np.linspace(0, 1, len(prices)), y=np.sort(prices), mode='markers', name='Data'))
fig.add_trace(go.Scatter(x=np.linspace(0, 1, len(prices)), y=dist.ppf(np.linspace(0, 1, len(prices)), *best_params), mode='lines', name='Best Fit'))
fig.update_layout(title=f'Q-Q Plot for {best_fit} Distribution', xaxis_title='Quantiles', yaxis_title='Values')
fig.show()

# Visualize the data with KDE
# Calculate KDE
kde = gaussian_kde(prices)
x_range = np.linspace(prices.min(), prices.max(), 1000)
kde_values = kde(x_range)

# Visualize the data with KDE
fig = go.Figure()
fig.add_trace(go.Histogram(x=prices, nbinsx=50, histnorm='probability density', name='Percent_Change'))
fig.add_trace(go.Scatter(x=x_range, y=kde_values, mode='lines', name='KDE'))

fig.update_layout(xaxis_title='Percent_Change', yaxis_title='Density')
fig.show()