import numpy as np
import pandas as pd
from scipy.stats import beta

# Load data from CSV
file_path = '/Users/jamal/projects/grodt/slack-trading/src/cmd/stats/clean_data_pdf/candles-COIN-5/percent_change-14400.csv'
df = pd.read_csv(file_path)
prices = df['Percent_Change'].dropna()

# Transform the data to fit within [0, 1]
min_price = prices.min()
max_price = prices.max()
scaled_prices = (prices - min_price) / (max_price - min_price)

# Fit the beta distribution
params = beta.fit(scaled_prices)
a, b, loc, scale = params

print(f"params: {params}")
print(f"max_price: {max_price}")
print(f"min_price: {min_price}")

# Calculate the cumulative probabilities for the given quantiles
# Transform the theoretical quantiles to fit within the same scale
theoretical_min = (-15 - min_price) / (max_price - min_price)
theoretical_max = (10 - min_price) / (max_price - min_price)

print(f"theoretical_min: {theoretical_min}")
print(f"theoretical_max: {theoretical_max}")

# Use the CDF (cumulative distribution function) to get the percentile values
percentile_min = beta.cdf(theoretical_min, a, b, loc, scale)
percentile_max = beta.cdf(theoretical_max, a, b, loc, scale)

# Print the results
print(f"Cumulative probability at quantile -15: {percentile_min * 100:.2f}%")
print(f"Cumulative probability at quantile 10: {percentile_max * 100:.2f}%")
print(f"Proportion of data between -15 and 10: {(percentile_max - percentile_min) * 100:.2f}%")
