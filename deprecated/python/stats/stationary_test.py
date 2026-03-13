import pandas as pd
from statsmodels.tsa.stattools import adfuller, kpss

# Sample sales data
sales = [100, 110, 108, 115, 120, 135, 166, 99, 177, 200, 205, 220, 300, 331, 331, 332, 350, 370]

# Convert to a pandas Series
sales_series = pd.Series(sales)

# Differencing the data
differenced_sales = sales_series.diff().dropna()

print(f'Differenced sales data: {differenced_sales}')

second_diff = differenced_sales.diff().dropna()

print(f'Second differenced sales data: {second_diff}')

# Augmented Dickey-Fuller test
adf_result = adfuller(second_diff)
print(f"ADF Statistic: {adf_result[0]}")
print(f"p-value: {adf_result[1]}")

# KPSS test
kpss_result = kpss(second_diff)
print(f"KPSS Statistic: {kpss_result[0]}")
print(f"p-value: {kpss_result[1]}")

# ADF Test:
# If the p-value is less than 0.05, we reject the null hypothesis, suggesting the data is stationary.
# KPSS Test:
# If the p-value is greater than 0.05, we do not reject the null hypothesis, suggesting the data is stationary.