import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
import seaborn as sns
from scipy.stats import (norm, lognorm, expon, t, beta, gamma, weibull_min, uniform,
                         chi2, logistic, laplace, pareto, cauchy, erlang, kstest, probplot)

# Load data from Excel
file_path = '/Users/jamal/projects/slack-trading/src/cmd/stats/clean_data_pdf_signals/candles-COIN-5/percent_change-14400.csv'  # Update with the path to your Excel file
df = pd.read_csv(file_path)
prices = df['Percent_Change'].dropna()

# Visualize the data
plt.figure(figsize=(12, 6))
sns.histplot(prices, kde=True, stat="density", linewidth=0)
plt.title('Histogram and KDE of Percent_Changes')
plt.xlabel('Percent_Change')
plt.ylabel('Density')
plt.show()

# Fit multiple distributions and evaluate the fit
distributions = {
    'Normal': norm,
    'Lognormal': lognorm,
    'Exponential': expon,
    't-Distribution': t,
    'Beta': beta,
    'Gamma': gamma,
    'Weibull': weibull_min,
    'Uniform': uniform,
    'Chi-Squared': chi2,
    'Logistic': logistic,
    'Laplace': laplace,
    'Pareto': pareto,
    'Cauchy': cauchy,
    'Erlang': erlang
}

results = {}

for name, distribution in distributions.items():
    try:
        # Fit the distribution to the data
        params = distribution.fit(prices)
        
        # Perform the Kolmogorov-Smirnov test
        D, p_value = kstest(prices, distribution.name, args=params)
        
        # Store the results
        results[name] = {'params': params, 'D': D, 'p_value': p_value}
    except Exception as e:
        print(f"Could not fit {name} distribution: {e}")

# Display the results
for name, result in results.items():
    print(f"{name} Distribution: D={result['D']:.4f}, p-value={result['p_value']:.4f}")

# Plot the Q-Q plot for the best fitting distribution
best_fit = min(results, key=lambda k: results[k]['D'])
best_params = results[best_fit]['params']
dist = distributions[best_fit]

# Q-Q plot
plt.figure(figsize=(12, 6))
probplot(prices, dist=dist, sparams=best_params, plot=plt)
plt.title(f'Q-Q Plot for {best_fit} Distribution')

print(f"The best fitting distribution is {best_fit} with parameters {best_params}")

plt.show()

