import pandas as pd
import os
import sys
import plotly.express as px

project_dir = os.getenv('PROJECTS_DIR')
if project_dir is None:
    raise ValueError('Please set the PROJECT_DIR environment variable')

if len(sys.argv) < 2:
    raise ValueError('Please provide the name of the CSV file in the /data directory as an argument')

csv_file = sys.argv[1]

file_path = os.path.join(project_dir, 'slack-trading', 'src', 'cmd', 'stats', 'data', csv_file)

# Load data
data = pd.read_csv(file_path)
data['Time'] = pd.to_datetime(data['Time'])  # Ensure 'Time' is datetime type

start_date = data['Time'].min()
start_date_formatted = start_date.strftime('%Y-%m-%d')
end_date = data['Time'].max()
end_date_formatted = end_date.strftime('%Y-%m-%d')

# Create the plot using Plotly
fig = px.line(data, x='Time', y='Close', title=f'Stock Price from {start_date_formatted} to {end_date_formatted}', labels={'Close': 'Stock Price', 'Time': 'Time'})

# Update layout for better readability
fig.update_layout(
    xaxis_title='Time',
    yaxis_title='Stock Price',
    xaxis=dict(
        rangeselector=dict(
            buttons=list([
                dict(count=1, label="1m", step="month", stepmode="backward"),
                dict(count=6, label="6m", step="month", stepmode="backward"),
                dict(step="all")
            ])
        ),
        rangeslider=dict(visible=True),
        type="date"
    )
)

# Show the plot
fig.show()
