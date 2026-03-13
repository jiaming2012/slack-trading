from lifelines import CoxPHFitter
from lifelines.utils import datetimes_to_durations
import pandas as pd
import matplotlib.pyplot as plt

def load_and_prepare_data(filepath):
    # Load data
    df = pd.read_csv(filepath)
    
    # Convert columns to datetime
    df['Signal Time'] = pd.to_datetime(df['Signal Time'])
    df['Event Time'] = pd.to_datetime(df['Event Time'])
    
    # Calculate durations and event occurred
    df['duration'], _ = datetimes_to_durations(df['Signal Time'], df['Event Time'], freq='D')
    return df

def fit_cox_model(df):
    cph = CoxPHFitter()
    cph.fit(df[['duration', 'Event Occurred', 'Stock Price', 'Target Price']], duration_col='duration', event_col='Event Occurred')
    return cph

def compute_survival_function(cph, df):
    survival_functions = cph.predict_survival_function(df[['Stock Price', 'Target Price']])
    return survival_functions

def main_cox(df):
    cph = fit_cox_model(df)
    survival_functions = compute_survival_function(cph, df)

    # print survival functions
    print(survival_functions)

    # print to csv
    survival_functions.to_csv('survival_functions.csv')

def get_label_from_filename(filename):
    # Convert filename stock_data_clean_10.csv into 10% gain
    print(filename.split('_')[-1])
    label = filename.split('_')[-1].split('.')[0]
    return label + '% Gain'

def main_kaplan_meier(dataframes):
    from lifelines import KaplanMeierFitter
    kmf = KaplanMeierFitter()

    # Convert filename stock_data_clean_10.csv into 10% gain


    # Plot survival functions
    for df in dataframes:
        kmf.fit(df['duration'], event_observed=df['Event Occurred'])
        kmf.plot_survival_function(label=get_label_from_filename(df.filename))

    # Add labels and title
    plt.xlabel('Time (hours)')
    plt.ylabel('Survival probability')
    plt.title('Kaplan-Meier Survival Curve')

    plt.show()


def find_input_file_names():
    import os

    # find all csv files in clean_data folder
    os.chdir('clean_data')
    files = os.listdir()
    input_files = [file for file in files if file.endswith('.csv')]
    return input_files

def main():
    file_names = find_input_file_names()
    dfs = []
    for file_name in file_names:
        df = load_and_prepare_data(file_name)
        df.filename = file_name
        dfs.append(df)

    main_kaplan_meier(dfs)

if __name__ == '__main__':
    main()
