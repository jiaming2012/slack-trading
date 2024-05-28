import pandas as pd
import glob
import numpy as np
import plotly.graph_objects as go
from plotly.subplots import make_subplots
from scipy.stats import gaussian_kde
from scipy.integrate import quad
import sys

# to run this script
# python pdf.py clean_data_pdf clean_data_pdf_signals candles-COIN-5

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

def generate_pdfs(dfs1, dfs2, min_value, max_value):
    rows_count = max(len(dfs1), len(dfs2))
    fig = make_subplots(rows=rows_count, cols=1)

    for dfs, y_offset in zip([dfs1, dfs2], [0, 0.05]):
        for counter, df in enumerate(dfs, 1):
            if len(df['Percent_Change']) < 2:
                continue

            # Calculate the PDFs using Gaussian Kernel Density Estimation
            kde = gaussian_kde(df['Percent_Change'])
            x_values = np.linspace(min_value, max_value, 100)
            pdf_values = kde(x_values)
            
            fig.add_trace(
                go.Scatter(x=x_values, y=pdf_values, mode='lines', name=df.label, line=dict(color=df.color), legendgroup=f'legend{counter}', legendwidth=1000, showlegend=True),
                row=counter, col=1
            )

            fig.update_xaxes(title_text=f'Percent Change, {df.number} mins', row=counter, col=1)  # Add your x-axis label here
            fig.update_yaxes(title_text="Density", row=counter, col=1)  # Add your y-axis label here

    return fig

# Input variables
inputDirectoryName1 = sys.argv[1]
inputDirectoryName2 = sys.argv[2]
inputStreamName = sys.argv[3]

# Input directory value
inputDirectory1 = f'{inputDirectoryName1}/{inputStreamName}'
inputDirectory2 = f'{inputDirectoryName2}/{inputStreamName}'

# Get a list of all .csv files in the clean_data_pdf folder
csv_files_1 = glob.glob(f'{inputDirectory1}/*.csv')
csv_files_2 = glob.glob(f'{inputDirectory2}/*.csv')

# Print the list of .csv files
prefix_1 = f'{inputDirectory1}/percent_change-'
prefix_2 = f'{inputDirectory2}/percent_change-'

colors_1 = ['red', 'green', 'purple', 'orange', 'black', 'brown', 'pink', 'cyan', 'magenta', 'blue']
colors_2 = ['blue', 'red', 'green', 'purple', 'orange', 'black', 'brown', 'pink', 'cyan', 'magenta']

# Sort the files by the number in the filename
csv_files_1_sorted = sorted(csv_files_1, key=lambda x: int(x[len(prefix_1):-4]))
csv_files_2_sorted = sorted(csv_files_2, key=lambda x: int(x[len(prefix_2):-4]))

min_value = None
max_value = None
start_time = None
end_time = None
dfs1 = []
dfs2 = []

for index, file in enumerate(csv_files_1_sorted):
    if file.startswith(prefix_1):
        number = int(file[len(prefix_1):-4])
        df = load_and_prepare_data(file)
        df.label = label = f'{number} mins'
        df.number = number
        df.color = colors_1[index % len(colors_1)]
        dfs1.append(df)

        # find the minimum values of the number in the filename
        min_value, max_value, start_time, end_time = update_min_max_values(df, min_value, max_value, start_time, end_time)

for index, file in enumerate(csv_files_2_sorted):
    if file.startswith(prefix_2):
        number = int(file[len(prefix_2):-4])
        df = load_and_prepare_data(file)
        df.label = label = f'uptrend, {number} mins'
        df.number = number
        df.color = colors_2[index % len(colors_2)]
        dfs2.append(df)

        # find the minimum values of the number in the filename
        min_value, max_value, start_time, end_time = update_min_max_values(df, min_value, max_value, start_time, end_time)


fig = generate_pdfs(dfs1, dfs2, min_value, max_value)

start_time = start_time.strftime('%Y-%m-%d %H:%M:%S')
end_time = end_time.strftime('%Y-%m-%d %H:%M:%S')

fig.update_layout(
    autosize=False,
    height=200*len(dfs1), 
    width=1000, 
    title_text=f'PDF of {inputStreamName} from {start_time} to {end_time}',
    showlegend=True
)

fig.show()
# plt.show()


# Define the threshold
# lower_threshold = 80
# upper_threshold = 110

# Calculate the probability that the stock price is above the threshold
# probability_above_threshold = quad(kde1, upper_threshold, np.inf)[0]

# print(f"The probability that the stock price is above {upper_threshold} is approximately {probability_above_threshold:.4f}")

# Calculate the probability that the stock price is below the lower threshold
# probability_below_threshold = quad(kde1, -np.inf, lower_threshold)[0]

# print(f"The probability that the stock price is below {lower_threshold} is approximately {probability_below_threshold:.4f}")


