import pandas as pd
import glob
import numpy as np
import matplotlib.pyplot as plt
from scipy.stats import gaussian_kde
from scipy.integrate import quad

def load_and_prepare_data(filepath):
    # Load data
    df = pd.read_csv(filepath)
    
    # Convert columns to datetime
    df['Time'] = pd.to_datetime(df['Time'])
    
    return df

def update_min_max_values(df, current_min, current_max, current_start_time, current_end_time):
    min_value = df['Percent_Change'].min()
    max_value = df['Percent_Change'].max()
    start_time = df['Time'].min()
    end_time = df['Time'].max()

    if current_min is None:
        current_min = min_value
    else:
        current_min = min(current_min, min_value)

    if current_max is None:
        current_max = max_value
    else:
        current_max = max(current_max, max_value)

    if current_start_time is None:
        current_start_time = start_time
    else:
        current_start_time = min(current_start_time, start_time)

    if current_end_time is None:
        current_end_time = end_time
    else:
        current_end_time = max(current_end_time, end_time)

    return current_min, current_max, current_start_time, current_end_time

def generate_pdfs(inputDirectory, dfs, min_value, max_value, start_time, end_time):
    plt.figure(figsize=(10, 6))

    for df in dfs:
        # Calculate the PDFs using Gaussian Kernel Density Estimation
        kde = gaussian_kde(df['Percent_Change'])
        x_values = np.linspace(min_value, max_value, 100)
        pdf_values1 = kde(x_values)

        # Plot the PDFs
        label = f'{df.label} mins'
        plt.plot(x_values, pdf_values1, label=label, color=df.color)
        # plt.plot(x_values, pdf_values2, label='PDF of Dataset 2', color='red')

    start_time = start_time.strftime('%Y-%m-%d %H:%M:%S')
    end_time = end_time.strftime('%Y-%m-%d %H:%M:%S')

    plt.title(f'PDF of {inputDirectory} from {start_time} to {end_time}')
    plt.xlabel('Percent Change')
    plt.ylabel('Density')
    plt.legend()
    plt.grid(True)

# Input variables
inputDirectory = 'candles-COIN-5'

# Get a list of all .csv files in the clean_data_pdf folder
csv_files = glob.glob(f'clean_data_pdf/{inputDirectory}/*.csv')

# Print the list of .csv files
prefix = f'clean_data_pdf/{inputDirectory}/percent_change-'

colors = ['blue', 'red', 'green', 'purple', 'orange', 'black', 'brown', 'pink', 'cyan', 'magenta']

# Sort the files by the number in the filename
csv_files_sorted = sorted(csv_files, key=lambda x: int(x[len(prefix):-4]))

min_value = None
max_value = None
start_time = None
end_time = None
dfs = []
for index, file in enumerate(csv_files_sorted):
    if file.startswith(prefix):
        number = int(file[len(prefix):-4])
        df = load_and_prepare_data(file)
        df.label = number
        df.color = colors[index % len(colors)]
        dfs.append(df)

        # find the minimum values of the number in the filename
        min_value, max_value, start_time, end_time = update_min_max_values(df, min_value, max_value, start_time, end_time)


generate_pdfs(inputDirectory, dfs, min_value, max_value, start_time, end_time)

plt.show()


# Define the threshold
# lower_threshold = 80
# upper_threshold = 110

# Calculate the probability that the stock price is above the threshold
# probability_above_threshold = quad(kde1, upper_threshold, np.inf)[0]

# print(f"The probability that the stock price is above {upper_threshold} is approximately {probability_above_threshold:.4f}")

# Calculate the probability that the stock price is below the lower threshold
# probability_below_threshold = quad(kde1, -np.inf, lower_threshold)[0]

# print(f"The probability that the stock price is below {lower_threshold} is approximately {probability_below_threshold:.4f}")


