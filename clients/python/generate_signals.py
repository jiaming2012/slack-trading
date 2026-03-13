from loguru import logger
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
from typing import List, Any, Tuple, Dict

def train_random_forest_models(df, feature_columns, target_columns):
    """
    Train Random Forest models to predict max_price_prediction and min_price_prediction.

    Args:
        df (pd.DataFrame): The DataFrame containing the data.
        feature_columns (list): List of feature column names.
        target_columns (list): List of target column names ('max_price_prediction', 'min_price_prediction').

    Returns:
        dict: Trained Random Forest models and their predictions.
    """
    models = {}
    predictions = {}
    info = []

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
        info.append((target, mse, r2, len(y_pred)))

    return models, predictions, info


def fetch_data(symbol: str, start_date: datetime, end_date: datetime) -> Tuple[pd.DataFrame, pd.DataFrame]:
    """
    Fetch historical stock data for a given symbol and date range.
    """
    
    ltf_data = fetch_polygon_stock_chart_aggregated(symbol, 5, 'minute', start_date, end_date)
    htf_data = fetch_polygon_stock_chart_aggregated(symbol, 60, 'minute', start_date, end_date)
    
    return ltf_data, htf_data

def fetch_data_and_add_supertrend_momentum_signal_features(symbol: str, start_date: datetime, end_date: datetime, min_max_window_in_hours: float):
    ltf_data, htf_data = fetch_data(symbol, start_date, end_date)
    feature_set_df = add_supertrend_momentum_signal_feature_set_v1(ltf_data, htf_data)
    target_set_df = add_supertrend_momentum_signal_target_set(feature_set_df, min_max_window_in_hours)
    return target_set_df

def add_supertrend_momentum_signal_target_set(df: pd.DataFrame, min_max_window_in_hours: float) -> pd.DataFrame:
    # Initialize new columns
    df['min_price_prediction'] = None
    df['max_price_prediction'] = None
    df['last_close_price'] = None
    df['min_price_prediction_time'] = None
    df['max_price_prediction_time'] = None
    df['last_close_price_time'] = None

    # Calculate min, max, and close prices within min_max_period_in_hours for rows where cross_below_80 is True
    for idx, row in df[df['stochrsi_cross_below_80']].iterrows():
        start_time = row['date']
        end_time = start_time + pd.Timedelta(hours=min_max_window_in_hours)
        mask = (df['date'] > start_time) & (df['date'] <= end_time)
        
        if not df.loc[mask].empty:
            min_price_idx = df.loc[mask, 'low'].idxmin()
            max_price_idx = df.loc[mask, 'high'].idxmax()
            close_price_row = df.loc[mask].iloc[-1] if not df.loc[mask].empty else None
            
            df.loc[idx, 'min_price_prediction'] = df.loc[min_price_idx, 'low']
            df.loc[idx, 'max_price_prediction'] = df.loc[max_price_idx, 'high']
            df.loc[idx, 'last_close_price'] = close_price_row['close'] if close_price_row is not None else None
            
            df.loc[idx, 'min_price_prediction_time'] = df.loc[min_price_idx, 'date']
            df.loc[idx, 'max_price_prediction_time'] = df.loc[max_price_idx, 'date']
            df.loc[idx, 'last_close_price_time'] = close_price_row['date'] if close_price_row is not None else None

    # Calculate min, max, and close prices within 1 day for rows where cross_above_20 is True
    for idx, row in df[df['stochrsi_cross_above_20']].iterrows():
        start_time = row['date']
        end_time = start_time + pd.Timedelta(hours=min_max_window_in_hours)
        mask = (df['date'] > start_time) & (df['date'] <= end_time)
        
        if not df.loc[mask].empty:
            min_price_idx = df.loc[mask, 'low'].idxmin()
            max_price_idx = df.loc[mask, 'high'].idxmax()
            close_price_row = df.loc[mask].iloc[-1] if not df.loc[mask].empty else None
            
            df.loc[idx, 'min_price_prediction'] = df.loc[min_price_idx, 'low']
            df.loc[idx, 'max_price_prediction'] = df.loc[max_price_idx, 'high']
            df.loc[idx, 'last_close_price'] = close_price_row['close'] if close_price_row is not None else None
            
            df.loc[idx, 'min_price_prediction_time'] = df.loc[min_price_idx, 'date']
            df.loc[idx, 'max_price_prediction_time'] = df.loc[max_price_idx, 'date']
            df.loc[idx, 'last_close_price_time'] = close_price_row['date'] if close_price_row is not None else None

    # Filter the DataFrame to include only rows with cross_below_80 or cross_above_20
    filtered_df = df[(df['stochrsi_cross_below_80']) | (df['stochrsi_cross_above_20'])]

    # Handle NaN values by filling them with 0
    filtered_df = filtered_df.fillna(0)
    
    return filtered_df

def add_supertrend_momentum_signal_feature_set_v2(ltf_data, htf_data, htf_data_daily, htf_data_weekly) -> pd.DataFrame:
    # Convert the 'Date' column to a datetime object
    exchange_tz = 'America/New_York'
    
    ltf_data['date'] = pd.to_datetime(ltf_data['datetime'], utc=True).dt.tz_convert(exchange_tz)
    htf_data['date'] = pd.to_datetime(htf_data['datetime'], utc=True).dt.tz_convert(exchange_tz)
    htf_data_daily['date'] = pd.to_datetime(htf_data_daily['datetime'], utc=True).dt.tz_convert(exchange_tz)
    htf_data_weekly['date'] = pd.to_datetime(htf_data_weekly['datetime'], utc=True).dt.tz_convert(exchange_tz)
    
    # Rename columns of supertrend_ltf
    htf_data = htf_data.rename(columns={
        'superT_50_3': 'superT_htf_50_3',
        'superD_50_3': 'superD_htf_50_3',
        'superS_50_3': 'superS_htf_50_3',
        'superL_50_3': 'superL_htf_50_3'
    })

    # Rename columns of supertrend_daily
    htf_data_daily = htf_data_daily.rename(columns={
        'superT_50_3': 'superT_htf_daily_50_3',
        'superD_50_3': 'superD_htf_daily_50_3',
        'superS_50_3': 'superS_htf_daily_50_3',
        'superL_50_3': 'superL_htf_daily_50_3'
    })
    
    # Rename columns of supertrend_weekly
    htf_data_weekly = htf_data_weekly.rename(columns={
        'superT_50_3': 'superT_htf_weekly_50_3',
        'superD_50_3': 'superD_htf_weekly_50_3',
        'superS_50_3': 'superS_htf_weekly_50_3',
        'superL_50_3': 'superL_htf_weekly_50_3'
    })
    
    df = pd.merge_asof(
        ltf_data,
        htf_data[['date', 'superT_htf_50_3', 'superD_htf_50_3', 'superS_htf_50_3', 'superL_htf_50_3']],
        on='date',
        direction='backward'  # Ensure each ltf_data row is matched with the most recent previous htf_df row
    )

    df = pd.merge_asof(
        df.sort_values('date'),
        htf_data_daily[['date', 'superT_htf_daily_50_3', 'superD_htf_daily_50_3', 'superS_htf_daily_50_3', 'superL_htf_daily_50_3']].sort_values('date'),
        on='date',
        direction='backward'  # Ensure each ltf_data row is matched with the most recent previous htf_df row
    )
    
    df = pd.merge_asof(
        df.sort_values('date'),
        htf_data_weekly[['date', 'superT_htf_weekly_50_3', 'superD_htf_weekly_50_3', 'superS_htf_weekly_50_3', 'superL_htf_weekly_50_3']].sort_values('date'),
        on='date',
        direction='backward'  # Ensure each ltf_data row is matched with the most recent previous htf_df row
    )
    
    return df

def add_supertrend_momentum_signal_feature_set_v1(ltf_data, htf_data) -> pd.DataFrame:
    # Fetch htf data
    supertrend = ta.supertrend(htf_data['High'], htf_data['Low'], htf_data['Close'], length=50, multiplier=3)
    htf_df = pd.concat([htf_data, supertrend], axis=1)

    # Fetch ltf data
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

     # Add exact values for the last 20 periods as features
    lag_features = pd.DataFrame({
        f'Close_{i}': ltf_df['Close'].shift(i) for i in range(1, 21)
    })
    
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

    return ltf_df
    
@dataclass
class SuperTrendMomentumSignalFactory:
    lag_features: int = 20
    feature_columns: List[str] = field(init=False)
    target_columns: List[str] = field(default_factory=lambda: ['max_price_prediction', 'min_price_prediction'])
    models: Any = None
    
    def __post_init__(self):
        self.feature_columns = [
            'open', 'high', 'low', 'close', 'volume',
            'stochrsi_k_14_14_3_3', 'stochrsi_d_14_14_3_3',
            'superT_50_3', 'superD_50_3', 'superL_50_3', 'superS_50_3',
            'superT_htf_50_3', 'superD_htf_50_3', 'superL_htf_50_3', 'superS_htf_50_3',
            'sma_50', 'sma_100', 'sma_200',
            'atr_14'
        ] + [f'close_lag_{i}' for i in range(1, self.lag_features+1)]

def get_dataframe_info(df: pd.DataFrame) -> str:
    start_date = df['date'].min()
    stop_date = df['date'].max()
    num_rows = len(df)
    avg_low = df['low'].mean()
    avg_high = df['high'].mean()
    
    return f"Start Date: {start_date}, Stop Date: {stop_date}, Number of Rows: {num_rows}, Average Low: {avg_low}, Average High: {avg_high}"

def new_supertrend_momentum_signal_factory(df: pd.DataFrame) -> Tuple[SuperTrendMomentumSignalFactory, str, Tuple[str, float, float]]:    
    factory = SuperTrendMomentumSignalFactory()
    
    factory.models, rf_predictions, training_info = train_random_forest_models(df, factory.feature_columns, factory.target_columns)
    
    df_info = get_dataframe_info(df)
    
    return factory, df_info, training_info

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
    
    start_date = datetime.datetime.strptime(args.start_date, '%Y-%m-%d')
    end_date = datetime.datetime.strptime(args.end_date, '%Y-%m-%d')
    
    filtered_df = fetch_data_and_add_supertrend_momentum_signal_features(args.symbol, start_date, end_date, min_max_window_in_hours=4)
    logger.info(f"generated {len(filtered_df)} {args.symbol} signals - from {start_date} to {end_date}")
    
    factory, info = new_supertrend_momentum_signal_factory(filtered_df)

    # Apply the newly generated model to a new dataset of future prices
    future_start_data = end_date + pd.Timedelta(days=1)
    future_end_data = future_start_data + pd.Timedelta(days=7)

    future_df = fetch_data_and_add_supertrend_momentum_signal_features(args.symbol, future_start_data, future_end_data, min_max_window_in_hours=4)
    future_df_features = future_df[factory.feature_columns]

    # Apply the trained models to the future data
    future_predictions_max = factory.models['max_price_prediction'].predict(future_df_features)
    future_predictions_min = factory.models['min_price_prediction'].predict(future_df_features)