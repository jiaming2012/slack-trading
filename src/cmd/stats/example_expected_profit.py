import numpy as np
import scipy.integrate as integrate
import plotly.graph_objs as go
from scipy.stats import beta, kurtosis as scipy_kurtosis
from fetch_options import fetch_options

# Parameters for the beta distribution
loc = 30.636946866327758
scale = 63.583766048450244
beta_dist = beta(a=1.4535158435210316, b=1.7364129668111365, loc=loc, scale=scale)
percent_change_pdf = beta_dist.pdf

# Calculate the mean
mean = beta_dist.mean()

# Calculate variance
variance = beta_dist.var()

# Calculate standard deviation
std_dev = np.sqrt(variance)

# Calculate skewness
skewness = beta_dist.stats(moments='s')

# Calculate kurtosis
kurtosis = beta_dist.stats(moments='k')

# Generate a range of percent changes
percent_changes = np.linspace(loc, loc + scale, 1000)

# Define the profit function for a call option based on percent change
def profit_function_call_percent(percent_change, stock_price, strike_price, premium):
    new_stock_price = stock_price * (1 + percent_change / 100)
    return max(new_stock_price - strike_price, 0) - premium

# Define the profit function for a call option based on percent change
def profit_function_put_percent(percent_change, stock_price, strike_price, premium):
    new_stock_price = stock_price * (1 + percent_change / 100)
    return max(strike_price - new_stock_price, 0) - premium


# Function to integrate: profit function * PDF
def call_integrand(percent_change):
    return profit_function_call_percent(percent_change, stock_price, strike_price, premium) * percent_change_pdf(percent_change)

def put_integrand(percent_change):
    return profit_function_put_percent(percent_change, stock_price, strike_price, premium) * percent_change_pdf(percent_change)

stock, options = fetch_options()

# Base stock price
stock_price = (stock.bid + stock.ask) / 2

print(f"Stock Price: {stock_price:.2f}")

for option in options:
    # Print the option
    print(option.description)

    # Parameters for the option
    strike_price = option.strike
    premium = option.ask

    # Integrate the expected profit over the range of percent changes
    if option.option_type == 'call':
        expected_profit, _ = integrate.quad(call_integrand, loc, loc + scale)
    elif option.option_type == 'put':
        expected_profit, _ = integrate.quad(put_integrand, loc, loc + scale)
    else:
        raise ValueError(f"Invalid option type: {option.option_type}")

    # Normalize by integrating the PDF over the same range
    # total_probability, _ = integrate.quad(percent_change_pdf, loc, loc + scale)

    # average_expected_profit = expected_profit / total_probability

    print(f"Expected Profit: {expected_profit:.2f}")

# Print the results
print("Beta Distribution Statistics:")
print(f"Mean: {mean:.2f}")
print(f"Standard Deviation: {std_dev:.2f}")
print(f"Variance: {variance:.2f}")
print(f"Skewness: {skewness:.2f}")
print(f"Kurtosis: {kurtosis:.2f}") # A normal distribution has a kurtosis of 3

# Calculate kurtosis for the left and right tails
median = beta_dist.median()
left_tail = beta_dist.rvs(size=1000, random_state=0)[beta_dist.rvs(size=1000, random_state=0) < median]
right_tail = beta_dist.rvs(size=1000, random_state=0)[beta_dist.rvs(size=1000, random_state=0) > median]
left_kurtosis = scipy_kurtosis(left_tail, fisher=False)  # Set fisher=False to get Pearson's kurtosis
right_kurtosis = scipy_kurtosis(right_tail, fisher=False)

# Print the results
print(f"Left Tail Kurtosis: {left_kurtosis:.2f}")
print(f"Right Tail Kurtosis: {right_kurtosis:.2f}")


# # Generate x values for plotting the percent changes PDF
# x_values = np.linspace(loc, loc + scale, 1000)
# pdf_values = percent_change_pdf(x_values)

# # Create the Plotly figure
# fig = go.Figure()

# # Plot the Expected Profit Area
# fig.add_trace(go.Scatter(x=x_values, y=[integrand(x) for x in x_values], 
#                          fill='tozeroy', name='Expected Profit Area', line=dict(color='orange', width=0), fillcolor='rgba(255,165,0,0.3)'))

# # Update layout
# fig.update_layout(
#     title='Probability Density Function of Percent Changes with Expected Profit Area',
#     xaxis_title='Percent Change',
#     yaxis_title='Density',
#     legend_title='Legend',
#     template='plotly_white'
# )

# # Show the plot
# fig.show()
