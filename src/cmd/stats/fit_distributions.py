import pandas as pd
import numpy as np
import plotly.express as px
import plotly.graph_objects as go
import sys
import os
import json
from scipy.stats import kstest, probplot, kurtosis as scipy_kurtosis
import distributions as dists

# to run this script:
# python expected_profit.py /Users/jamal/projects/slack-trading/src/cmd/stats/transform_data/supertrend_4h_1h_stoch_rsi_15m_up/candles-SPX-15/best_fit_percent_change-1440.json

# Input variable
inDir = sys.argv[1]

print(f"Loading data from {inDir}")

# Load data from Excel
df = pd.read_csv(inDir)
prices = df['Percent_Change'].dropna()

# Visualize the data with KDE
# Calculate KDE
from scipy.stats import gaussian_kde

kde = gaussian_kde(prices)
x_range = np.linspace(prices.min(), prices.max(), 1000)
kde_values = kde(x_range)

# Visualize the data with KDE
fig = go.Figure()
fig.add_trace(go.Histogram(x=prices, nbinsx=50, histnorm='probability density', name='Percent_Change'))
fig.add_trace(go.Scatter(x=x_range, y=kde_values, mode='lines', name='KDE'))

fig.update_layout(xaxis_title='Percent_Change', yaxis_title='Density')
fig.show()


results = {}
for name, distribution in dists.distributions.items():
    try:
        # Fit the distribution to the data
        params = distribution.fit(prices)
        
        # Perform the Kolmogorov-Smirnov test
        D, p_value = kstest(prices, distribution.name, args=params)
        
        # Store the results
        results[name] = {'params': params, 'D': D, 'p_value': p_value}
    except Exception as e:
        print(f"Could not fit {name} distribution: {e}")

print(f"Results: {results}")

# Display the results
for name, result in results.items():
    print(f"{name} Distribution: D={result['D']:.4f}, p-value={result['p_value']:.4f}")

# Plot the Q-Q plot for the best fitting distribution
best_fit = min(results, key=lambda k: results[k]['D'])
best_params = results[best_fit]['params']
dist = dists.distributions[best_fit]

# Q-Q plot
fig = go.Figure()
fig.add_trace(go.Scatter(x=np.linspace(0, 1, len(prices)), y=np.sort(prices), mode='markers', name='Data'))
fig.add_trace(go.Scatter(x=np.linspace(0, 1, len(prices)), y=dist.ppf(np.linspace(0, 1, len(prices)), *best_params), mode='lines', name='Best Fit'))
fig.update_layout(title=f'Q-Q Plot for {best_fit} Distribution', xaxis_title='Quantiles', yaxis_title='Values')

print(f"The best fitting distribution is {best_fit} with parameters {best_params}")

# Export the best fit distribution and its parameters to a json file
output = {'distribution': best_fit, 'params': best_params}

# Go back one directory
outDir = os.path.dirname(inDir)
filename, _ = os.path.splitext(os.path.basename(inDir))

print(f"Exporting results to {outDir}")

with open(f'{outDir}/best_fit_{filename}.json', 'w') as f:
    json.dump(output, f)

distribution = dists.distributions[best_fit](*best_params)

# Calculate the mean
mean = distribution.mean()

# Calculate variance
variance = distribution.var()

# Calculate standard deviation
std_dev = np.sqrt(variance)

# Calculate skewness
skewness = distribution.stats(moments='s')

# Calculate kurtosis
kurtosis = distribution.stats(moments='k')

# Print the results
print(f"Distribution Statistics:")
print(f"Mean: {mean:.2f}")
print(f"Standard Deviation: {std_dev:.2f}")
print(f"Variance: {variance:.2f}")
print(f"Skewness: {skewness:.2f}")
print(f"Kurtosis: {kurtosis:.2f}") # A normal distribution has a kurtosis of 3

# Calculate kurtosis for the left and right tails
median = distribution.median()
random_variates = distribution.rvs(size=10000, random_state=0)

left_tail = random_variates[random_variates < median]
right_tail = random_variates[random_variates > median]
left_kurtosis = scipy_kurtosis(left_tail, fisher=False)  # Set fisher=False to get Pearson's kurtosis
right_kurtosis = scipy_kurtosis(right_tail, fisher=False)

# Print the results
print(f"Left Tail Kurtosis: {left_kurtosis:.2f}")
print(f"Right Tail Kurtosis: {right_kurtosis:.2f}")

fig.show()
