import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
from scipy.stats import gaussian_kde
from scipy.integrate import quad

# Generate random stock ticks data (for demonstration purposes)
np.random.seed(42)
np.random.seed(42)
stock_ticks1 = np.random.normal(loc=100, scale=10, size=1000)  # Mean = 100, Std = 10, Sample size = 1000
stock_ticks2 = np.random.normal(loc=105, scale=15, size=1000)  # Mean = 105, Std = 15, Sample size = 1000

# Convert to pandas DataFrame
df1 = pd.DataFrame(stock_ticks1, columns=['Price'])
df2 = pd.DataFrame(stock_ticks2, columns=['Price'])

# Calculate the PDFs using Gaussian Kernel Density Estimation
kde1 = gaussian_kde(df1['Price'])
kde2 = gaussian_kde(df2['Price'])
x_values = np.linspace(min(min(df1['Price']), min(df2['Price'])), max(max(df1['Price']), max(df2['Price'])), 1000)
pdf_values1 = kde1(x_values)
pdf_values2 = kde2(x_values)

# Plot the PDFs
plt.figure(figsize=(10, 6))
plt.plot(x_values, pdf_values1, label='PDF of Dataset 1', color='blue')
plt.plot(x_values, pdf_values2, label='PDF of Dataset 2', color='red')
plt.title('Probability Density Functions of Stock Ticks')
plt.xlabel('Stock Price')
plt.ylabel('Density')
plt.legend()
plt.grid(True)

# Define the threshold
lower_threshold = 80
upper_threshold = 110

# Calculate the probability that the stock price is above the threshold
probability_above_threshold = quad(kde1, upper_threshold, np.inf)[0]

print(f"The probability that the stock price is above {upper_threshold} is approximately {probability_above_threshold:.4f}")

# Calculate the probability that the stock price is below the lower threshold
probability_below_threshold = quad(kde1, -np.inf, lower_threshold)[0]

print(f"The probability that the stock price is below {lower_threshold} is approximately {probability_below_threshold:.4f}")

plt.show()
