import pandas as pd
import matplotlib.pyplot as plt
import matplotlib.dates as mdates

# Load data
data = pd.read_csv('stock_data.csv')
data['Time'] = pd.to_datetime(data['Time'])  # Ensure 'Time' is datetime type

# Plot
fig, ax = plt.subplots()
ax.plot(data['Time'], data['Stock Price'], marker='', linestyle='-', color='blue')
plt.title('Stock Price Over Time')
plt.xlabel('Time')
plt.ylabel('Stock Price')
plt.grid(True)

# Set the locator
locator = mdates.AutoDateLocator()
# formatter = mdates.ConciseDateFormatter(locator)
ax.xaxis.set_major_locator(locator)
# ax.xaxis.set_major_formatter(formatter)

# Rotate date labels for better visibility
plt.xticks(rotation=45)
plt.tight_layout()
plt.savefig('stock_price_plot.png')  # Save the plot to a file
plt.show()
