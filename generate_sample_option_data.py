import pandas as pd
import numpy as np
from datetime import timedelta
import io

# Original CSV data
csv_data = """
ticker,tradeDate,expirDate,dte,strike,stockPrice,callVolume,callOpenInterest,callBidSize,callAskSize,putVolume,putOpenInterest,putBidSize,putAskSize,callBidPrice,callValue,callAskPrice,putBidPrice,putValue,putAskPrice,callBidIv,callMidIv,callAskIv,smvVol,putBidIv,putMidIv,putAskIv,residualRate,delta,gamma,theta,vega,rho,phi,driftlessTheta,callSmvVol,putSmvVol,extSmvVol,extCallValue,extPutValue,spotPrice,quoteDate,updatedAt,snapShotEstTime,snapShotDate,expiryTod,tickerId,monthId
IWM,2022-06-08,2022-09-16,101,203,149.67,150,28206,253,510,7,18438,124,104,5.8,8.860204002795059,8.9,15.55,15.641054396787531,10.7,0.3063158772183223,0.30801686256479954,0.3097178479112768,0.308,0.3038649107227709,0.3064286008767878,0.3089922910308047,-0.008209120374803128,0.38161255239927655,0.015355889799308844,-0.047622767388992515,0.2939141986686304,0.13545210863618393,-0.15118639086979158,-0.0459052133276907,0.3083639997493957,0.3069773808630928,0.3215480409289392,6.247634998832532,16.067895502105014,149.67,2022-06-08T13:59:50Z,2022-06-08T13:59:51Z,2022-06-08T14:00:01Z,2022-06-08T14:00:01Z,pm,101594,9
IWM,2022-06-08,2022-09-16,101,203,149.45,158,28206,1,215,7,18438,229,102,5.75,8.78583812885739,8.8,15.7,15.782968253643883,10.85,0.30745849466065145,0.3083088916342891,0.3091592886079268,0.308,0.30432705746682837,0.30689074762084534,0.3094544377748623,-0.008209120374803128,0.37823425664342863,0.015355889799308844,-0.047622767388992515,0.2939141986686304,0.13545210863618393,-0.15118639086979158,-0.0459052133276907,0.3086775601134979,0.3071631227329991,0.3215480409289392,6.1640518498378345,16.204312353110314,149.45,2022-06-08T14:00:55Z,2022-06-08T14:00:56Z,2022-06-08T14:01:01Z,2022-06-08T14:01:01Z,pm,101594,9
IWM,2022-06-08,2022-09-16,101,203,149.12,159,28206,152,1299,13,18438,234,132,5.6,8.66120101953646,8.7,15.9,15.991901944264272,11.05,0.3065732423106423,0.30827422765711954,0.3099752130035968,0.308,0.3041222758150914,0.30668596596910835,0.3092496561231253,-0.008209120374803128,0.37316681300965693,0.015355889799308844,-0.047622767388992515,0.2939141986686304,0.13545210863618393,-0.15118639086979158,-0.0459052133276907,0.30865528305906204,0.3072637172770291,0.3215480409289392,6.040070673345081,16.410331176617547,149.12,2022-06-08T14:02:00Z,2022-06-08T14:02:01Z,2022-06-08T14:02:02Z,2022-06-08T14:02:02Z,pm,101594,9
"""

# Read CSV data into a DataFrame
df = pd.read_csv(io.StringIO(csv_data))

# Function to generate random variations
def generate_variations(df, n_rows):
    new_rows = []
    for i in range(n_rows):
        row = df.sample(n=1).copy().iloc[0]
        
        # Randomly vary some numeric fields
        row['callBidPrice'] = row['callBidPrice'] + np.random.uniform(-1, 1)
        row['putBidPrice'] = row['putBidPrice'] + np.random.uniform(-1, 1)
        row['callVolume'] = row['callVolume'] + np.random.randint(-10, 10)
        row['putVolume'] = row['putVolume'] + np.random.randint(-10, 10)
        
        # Append the new row to the list
        new_rows.append(row)
    
    return pd.DataFrame(new_rows)

# Generate 1000 rows
df_extended = generate_variations(df, 1000)

# Initialize the starting timestamp
start_time = pd.to_datetime('2022-06-08T14:00:01Z')

# Create the snapShotEstTime and snapShotDate columns with one minute increments
df_extended['snapShotEstTime'] = [start_time + timedelta(minutes=i) for i in range(len(df_extended))]
df_extended['snapShotDate'] = [start_time + timedelta(minutes=i) for i in range(len(df_extended))]

# Convert to string format
df_extended['snapShotEstTime'] = df_extended['snapShotEstTime'].dt.strftime('%Y-%m-%dT%H:%M:%SZ')
df_extended['snapShotDate'] = df_extended['snapShotDate'].dt.strftime('%Y-%m-%dT%H:%M:%SZ')

# Display the first few rows to check the increments
df_extended[['snapShotEstTime', 'snapShotDate']].head()

# Write the DataFrame to a new CSV file
df_extended.to_csv('sample_option_data_extended.csv', index=False)
