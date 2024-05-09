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

def main_kaplan_meier(df, subtitle):
    from lifelines import KaplanMeierFitter
    kmf = KaplanMeierFitter()
    kmf.fit(df['duration'], event_observed=df['Event Occurred'])
    kmf.plot_survival_function()

    # Add labels and title
    plt.xlabel('Time (hours)')
    plt.ylabel('Survival probability')
    plt.title('Kaplan-Meier Survival Curve for {}'.format(subtitle))

    plt.show()

def main():
    filepath = 'stock_data_clean.csv'
    df = load_and_prepare_data(filepath)
    
    # print dataframe
    print(df.head())

    # run
    main_kaplan_meier(df, "25% Gain")

if __name__ == '__main__':
    main()
