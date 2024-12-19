import argparse
import datetime
import pandas as pd
import pandas_ta as ta
import numpy as np
from utils import fetch_polygon_stock_chart_aggregated

from sklearn.ensemble import RandomForestRegressor, GradientBoostingRegressor
from sklearn.model_selection import train_test_split
from sklearn.metrics import mean_squared_error, r2_score

from dataclasses import dataclass, field
from typing import List, Any

def train_random_forest_models(df, feature_columns, target_columns):
    """
    Train Random Forest models to predict max_price_1d and min_price_1d.

    Args:
        df (pd.DataFrame): The DataFrame containing the data.
        feature_columns (list): List of feature column names.
        target_columns (list): List of target column names ('max_price_1d', 'min_price_1d').

    Returns:
        dict: Trained Random Forest models and their predictions.
    """
    models = {}
    predictions = {}

    for target in target_columns:
        # Split the data
        X = df[feature_columns]
        y = df[target]
        X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=42)

        # Train Random Forest Regressor
        model = RandomForestRegressor(n_estimators=150, random_state=42)
        model.fit(X_train, y_train)
        y_pred = model.predict(X_test)
        
        # Store the model and predictions
        models[target] = model
        predictions[target] = (X_test, y_test, y_pred)

        # Evaluate the model
        mse = mean_squared_error(y_test, y_pred)
        r2 = r2_score(y_test, y_pred)
        print(f"Model for {target}:")
        print(f"Mean Squared Error: {mse}")
        print(f"R2 Score: {r2}")
        print("-" * 40)

    return models, predictions

def fetch_data_and_add_features(symbol: str, start_date: datetime, end_date: datetime):
    #Inputs
    min_max_period_in_hours = 4
    
    # Fetch htf data
    htf_data = fetch_polygon_stock_chart_aggregated(symbol, 60, 'minute', start_date, end_date)
    supertrend = ta.supertrend(htf_data['High'], htf_data['Low'], htf_data['Close'], length=50, multiplier=3)
    htf_df = pd.concat([htf_data, supertrend], axis=1)

    # Fetch ltf data
    ltf_data = fetch_polygon_stock_chart_aggregated(symbol, 5, 'minute', start_date, end_date)
    stochrsi = ta.stochrsi(ltf_data['Close'], rsi_length=14, stoch_length=14, k=3, d=3)
    supertrend_ltf = ta.supertrend(ltf_data['High'], ltf_data['Low'], ltf_data['Close'], length=50, multiplier=3)
    
    # Rename columns of supertrend_ltf
    supertrend_ltf = supertrend_ltf.rename(columns={
        'SUPERT_50_3.0': 'SUPERT_LTF_50_3.0',
        'SUPERTd_50_3.0': 'SUPERTd_LTF_50_3.0',
        'SUPERTl_50_3.0': 'SUPERTl_LTF_50_3.0', 
        'SUPERTs_50_3.0': 'SUPERTs_LTF_50_3.0'
    })
    
    ltf_df = pd.concat([ltf_data, stochrsi, supertrend_ltf], axis=1)
    
    # Add moving averages as features
    ma_features = pd.DataFrame({
        'MA_50': ltf_df['Close'].rolling(window=50).mean(),
        'MA_100': ltf_df['Close'].rolling(window=100).mean(),
        'MA_200': ltf_df['Close'].rolling(window=200).mean()
    })

    # Add exact values for the last 50 periods as features
    # lag_features = pd.DataFrame({
    #     f'High_{i}': ltf_df['High'].shift(i) for i in range(1, 51)
    # }).join(pd.DataFrame({
    #     f'Low_{i}': ltf_df['Low'].shift(i) for i in range(1, 51)
    # })).join(pd.DataFrame({
    #     f'Close_{i}': ltf_df['Close'].shift(i) for i in range(1, 51)
    # })).join(pd.DataFrame({
    #     f'Open_{i}': ltf_df['Open'].shift(i) for i in range(1, 51)
    # }))
     # Add exact values for the last 20 periods as features
    lag_features = pd.DataFrame({
        f'High_{i}': ltf_df['High'].shift(i) for i in range(1, 21)
    }).join(pd.DataFrame({
        f'Low_{i}': ltf_df['Low'].shift(i) for i in range(1, 21)
    })).join(pd.DataFrame({
        f'Close_{i}': ltf_df['Close'].shift(i) for i in range(1, 21)
    })).join(pd.DataFrame({
        f'Open_{i}': ltf_df['Open'].shift(i) for i in range(1, 21)
    }))
    
    # Add ATR as a feature
    atr = ta.atr(ltf_df['High'], ltf_df['Low'], ltf_df['Close'], length=14)
    atr_features = pd.DataFrame({
        'ATR_14': atr
    })
    
    # Concatenate all new features at once
    ltf_df = pd.concat([ltf_df, ma_features, lag_features, atr_features], axis=1)
        
    # Merge supertrend from htf_df into ltf_df
    ltf_df = pd.merge_asof(
        ltf_df.sort_values('Date'),
        htf_df[['Date', 'SUPERT_50_3.0', 'SUPERTd_50_3.0', 'SUPERTl_50_3.0', 'SUPERTs_50_3.0']].sort_values('Date'),
        on='Date',
        direction='backward'  # Ensure each ltf_df row is matched with the most recent previous htf_df row
    )

    # Identify stochastic RSI crossovers
    ltf_df['cross_below_80'] = (ltf_df['STOCHRSId_14_14_3_3'] > 80) & (ltf_df['STOCHRSIk_14_14_3_3'].shift(1) >= ltf_df['STOCHRSId_14_14_3_3'].shift(1)) & (ltf_df['STOCHRSIk_14_14_3_3'] < ltf_df['STOCHRSId_14_14_3_3'])
    ltf_df['cross_above_20'] = (ltf_df['STOCHRSId_14_14_3_3'] < 20) & (ltf_df['STOCHRSIk_14_14_3_3'].shift(1) <= ltf_df['STOCHRSId_14_14_3_3'].shift(1)) & (ltf_df['STOCHRSIk_14_14_3_3'] > ltf_df['STOCHRSId_14_14_3_3'])

    # Initialize new columns
    ltf_df['min_price_1d'] = None
    ltf_df['max_price_1d'] = None
    ltf_df['close_price_1d'] = None
    ltf_df['min_price_1d_time'] = None
    ltf_df['max_price_1d_time'] = None
    ltf_df['close_price_1d_time'] = None

    # Calculate min, max, and close prices within 1 day for rows where cross_below_80 is True
    for idx, row in ltf_df[ltf_df['cross_below_80']].iterrows():
        start_time = row['Date']
        end_time = start_time + pd.Timedelta(hours=min_max_period_in_hours)
        mask = (ltf_df['Date'] > start_time) & (ltf_df['Date'] <= end_time)
        
        if not ltf_df.loc[mask].empty:
            min_price_idx = ltf_df.loc[mask, 'Low'].idxmin()
            max_price_idx = ltf_df.loc[mask, 'High'].idxmax()
            close_price_row = ltf_df.loc[mask].iloc[-1] if not ltf_df.loc[mask].empty else None
            
            ltf_df.loc[idx, 'min_price_1d'] = ltf_df.loc[min_price_idx, 'Low']
            ltf_df.loc[idx, 'max_price_1d'] = ltf_df.loc[max_price_idx, 'High']
            ltf_df.loc[idx, 'close_price_1d'] = close_price_row['Close'] if close_price_row is not None else None
            
            ltf_df.loc[idx, 'min_price_1d_time'] = ltf_df.loc[min_price_idx, 'Date']
            ltf_df.loc[idx, 'max_price_1d_time'] = ltf_df.loc[max_price_idx, 'Date']
            ltf_df.loc[idx, 'close_price_1d_time'] = close_price_row['Date'] if close_price_row is not None else None

    # Calculate min, max, and close prices within 1 day for rows where cross_above_20 is True
    for idx, row in ltf_df[ltf_df['cross_above_20']].iterrows():
        start_time = row['Date']
        end_time = start_time + pd.Timedelta(hours=min_max_period_in_hours)
        mask = (ltf_df['Date'] > start_time) & (ltf_df['Date'] <= end_time)
        
        if not ltf_df.loc[mask].empty:
            min_price_idx = ltf_df.loc[mask, 'Low'].idxmin()
            max_price_idx = ltf_df.loc[mask, 'High'].idxmax()
            close_price_row = ltf_df.loc[mask].iloc[-1] if not ltf_df.loc[mask].empty else None
            
            ltf_df.loc[idx, 'min_price_1d'] = ltf_df.loc[min_price_idx, 'Low']
            ltf_df.loc[idx, 'max_price_1d'] = ltf_df.loc[max_price_idx, 'High']
            ltf_df.loc[idx, 'close_price_1d'] = close_price_row['Close'] if close_price_row is not None else None
            
            ltf_df.loc[idx, 'min_price_1d_time'] = ltf_df.loc[min_price_idx, 'Date']
            ltf_df.loc[idx, 'max_price_1d_time'] = ltf_df.loc[max_price_idx, 'Date']
            ltf_df.loc[idx, 'close_price_1d_time'] = close_price_row['Date'] if close_price_row is not None else None

    # Filter the DataFrame to include only rows with cross_below_80 or cross_above_20
    filtered_df = ltf_df[(ltf_df['cross_below_80']) | (ltf_df['cross_above_20'])]

    # Handle NaN values by filling them with 0
    filtered_df = filtered_df.fillna(0)
    
    return filtered_df
    
def analyze_data(title, y_test, y_pred):
    # Create a DataFrame with predictions and actual values
    results_df = pd.DataFrame({
        'Actual': y_test,
        'Predicted': y_pred
    })

    # Print the DataFrame
    print(f"Results for {title}:")
    print(results_df.head(20))

    # Calculate the standard deviation of the residuals for max_price_1d
    residuals_max = y_test - y_pred
    std_dev_max = np.std(residuals_max)
    print(f"Standard Deviation of Residuals for {title}: {std_dev_max}")
    
    mse = mean_squared_error(y_test, y_pred)
    r2 = r2_score(y_test, y_pred)

    print(f"Mean Squared Error: {mse}")
    print(f"R2 Score: {r2}")
    print("-" * 40)
    
@dataclass
class SuperTrendMomentumSignalFactory:
    lag_features: int = 20
    feature_columns: List[str] = field(init=False)
    target_columns: List[str] = field(default_factory=lambda: ['max_price_1d', 'min_price_1d'])
    models: Any = None
    
    def __post_init__(self):
        self.feature_columns = [
            'Open', 'High', 'Low', 'Close', 'Volume',
            'STOCHRSIk_14_14_3_3', 'STOCHRSId_14_14_3_3',
            'SUPERT_50_3.0', 'SUPERTd_50_3.0', 'SUPERTl_50_3.0', 'SUPERTs_50_3.0',
            'SUPERT_LTF_50_3.0', 'SUPERTd_LTF_50_3.0', 'SUPERTl_LTF_50_3.0', 'SUPERTs_LTF_50_3.0',
            'MA_50', 'MA_100', 'MA_200',
            'ATR_14'
        ] + [f'High_{i}' for i in range(1, self.lag_features+1)] + [f'Low_{i}' for i in range(1, self.lag_features+1)] + [f'Close_{i}' for i in range(1, self.lag_features+1)] + [f'Open_{i}' for i in range(1, self.lag_features+1)]

def new_supertrend_momentum_signal_factory(symbol: str, start_date: str, end_date: str) -> SuperTrendMomentumSignalFactory:
    start_date = datetime.datetime.strptime(args.start_date, '%Y-%m-%d')
    end_date = datetime.datetime.strptime(args.end_date, '%Y-%m-%d')
    filtered_df = fetch_data_and_add_features(symbol, start_date, end_date)

    print(f"generated {len(filtered_df)} {symbol} signals - from {start_date} to {end_date}")
    
    factory = SuperTrendMomentumSignalFactory()
    
    # Train models
    print("Training Random Forest models...")
    factory.models, predictions = train_random_forest_models(filtered_df, factory.feature_columns, factory.target_columns)
    print("Training complete.")
        
    # Analyze Random Forest predictions
    for target in factory.target_columns:
        X_test, y_test, y_pred = predictions[target]
        analyze_data(f"RandomForest - {target}", y_test, y_pred)
        
    # Todo: create criteria for rejecting the model
    
    return factory

if __name__ == '__main__':
    # Parse arguments
    parser = argparse.ArgumentParser(description='generate signals for a model.')
    parser.add_argument('--start-date', type=str, help='The start date of the signals', required=True)
    parser.add_argument('--end-date', type=str, help='The end date of the signals', required=True)
    parser.add_argument('--symbol', type=str, help='The symbol to generate signals for', required=True)
    parser.add_argument('--eventFn', type=str, help='The event function to use for generating signals', required=True)

    args = parser.parse_args()

    # Set display options to print all rows and columns
    pd.set_option('display.max_rows', None)
    pd.set_option('display.max_columns', None)
    
    factory = new_supertrend_momentum_signal_factory(args.symbol, args.start_date, args.end_date)

    # Apply the newly generated model to a new dataset of future prices
    start_date = datetime.datetime.strptime(args.start_date, '%Y-%m-%d')
    end_date = datetime.datetime.strptime(args.end_date, '%Y-%m-%d')
    future_start_data = end_date + pd.Timedelta(days=1)
    future_end_data = future_start_data + pd.Timedelta(days=7)

    future_df = fetch_data_and_add_features(args.symbol, future_start_data, future_end_data)
    future_df_features = future_df[factory.feature_columns]

    # Apply the trained models to the future data
    future_predictions_max = factory.models['max_price_1d'].predict(future_df_features)
    future_predictions_min = factory.models['min_price_1d'].predict(future_df_features)

    analyze_data('future_max_price_1d', future_df['max_price_1d'], future_predictions_max)
    analyze_data('future_min_price_1d', future_df['min_price_1d'], future_predictions_min)