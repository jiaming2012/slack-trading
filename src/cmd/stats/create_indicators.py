import argparse
import sys
import json
import pandas as pd
import pandas_ta as ta

def calculate_supertrend(df) -> pd.DataFrame:
    supertrend = ta.supertrend(df['high'], df['low'], df['close'], length=50, multiplier=3)
    df = pd.concat([df, supertrend], axis=1)
    return df

def calculate_stochrsi(df) -> pd.DataFrame:
    stochrsi = ta.stochrsi(df['close'], rsi_length=14, stoch_length=14, k=3, d=3)
    df = pd.concat([df, stochrsi], axis=1)
    return df

def calculate_moving_averages(df) -> pd.DataFrame:
    sma_50 = ta.sma(df['close'], length=50)
    sma_100 = ta.sma(df['close'], length=100)
    sma_200 = ta.sma(df['close'], length=200)
    df = pd.concat([df, sma_50, sma_100, sma_200], axis=1)
    return df

def calculate_lag_features(df, count: int) -> pd.DataFrame:
    lags = pd.DataFrame({
        f'close_lag_{i}': df['close'].shift(i) for i in range(1, count+1)
    })
    
    df = pd.concat([df, lags], axis=1)
    
    return df
    
def calculate_atr(df) -> pd.DataFrame:
    atr = ta.atr(df['high'], df['low'], df['close'], length=14)
    df = pd.concat([df, atr], axis=1)
    return df

def calculate_stochrsi_cross_above_20(df) -> pd.DataFrame:
    
    
    try:
        df['stochrsi_cross_above_20'] = (df['STOCHRSId_14_14_3_3'] < 20) & (df['STOCHRSIk_14_14_3_3'].shift(1) <= df['STOCHRSId_14_14_3_3'].shift(1)) & (df['STOCHRSIk_14_14_3_3'] > df['STOCHRSId_14_14_3_3'])
    except Exception as e:
        print(df['STOCHRSId_14_14_3_3'])
        print(df['STOCHRSIk_14_14_3_3'])
        
        raise e
    return df

def calculate_stochrsi_cross_below_80(df) -> pd.DataFrame:
    df['stochrsi_cross_below_80'] = (df['STOCHRSId_14_14_3_3'] > 80) & (df['STOCHRSIk_14_14_3_3'].shift(1) >= df['STOCHRSId_14_14_3_3'].shift(1)) & (df['STOCHRSIk_14_14_3_3'] < df['STOCHRSId_14_14_3_3'])
    return df

def main():
    args = argparse.ArgumentParser()
    args.add_argument('--indicators', type=str, nargs='+', required=True, help="List of indicators to calculate")
    
    args = args.parse_args()

    # Read candles from standard input
    input_data = sys.stdin.read()
    candles = json.loads(input_data)
    
    df = pd.DataFrame(candles)
    
    # Make a list of indicators to calculate
    derived_indicators = []
    for indicator in args.indicators:    
        if indicator == 'supertrend':
            df = calculate_supertrend(df)
        elif indicator == 'stochrsi':
            df = calculate_stochrsi(df)
        elif indicator == 'moving_averages':
            df = calculate_moving_averages(df)
        elif indicator == 'lag_features':
            df = calculate_lag_features(df, 20)
        elif indicator == 'atr':
            df = calculate_atr(df)
        elif indicator == 'stochrsi_cross_above_20':
            derived_indicators.append('stochrsi_cross_above_20')
        elif indicator == 'stochrsi_cross_below_80':
            derived_indicators.append('stochrsi_cross_below_80')
        else:
            raise Exception(f"Unsupported indicator: {indicator}")
    
    # Convert NaN values to 0
    df = df.fillna(0)
    
    for indicator in derived_indicators:
        if indicator == 'stochrsi_cross_above_20':
            df = calculate_stochrsi_cross_above_20(df)
        elif indicator == 'stochrsi_cross_below_80':
            df = calculate_stochrsi_cross_below_80(df)
        else:
            raise Exception(f"Unsupported derived indicator: {indicator}")
    
    # Convert DataFrame to JSON and print to standard output
    df_json = df.to_json(orient='records')
    print(df_json)
    
main()