import matplotlib.pyplot as plt
import pandas as pd

# Load data from CSV
data = pd.read_csv('stock_data.csv')

# Plotting
plt.figure(figsize=(10, 5))
plt.plot(data['Time Step'], data['Stock Price'], marker='', linestyle='-', color='blue')
plt.title('Stock Price Over Time')
plt.xlabel('Time Step')
plt.ylabel('Stock Price')
plt.grid(True)
plt.savefig('stock_price_plot.png')  # Save the plot to a file
plt.show()
