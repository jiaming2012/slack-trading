import requests
import pandas as pd

def FetchPolygonDataframe(symbol, start, end, multiplier, timespan):
    url = 'http://localhost:8080/data/polygon?symbol={}&from={}&to={}&multiplier={}&timespan={}'.format(symbol, start, end, multiplier, timespan)
    response = requests.get(url)
    
    data = response.json()
        
    if response.status_code != 200:
        raise Exception('Error fetching data from Polygon')
        
    # Convert the JSON response into a DataFrame
    df = pd.DataFrame(data)

    # Convert the 'Datetime' column to Pandas datetime, recognizing the RFC 3339 format
    df['Datetime'] = pd.to_datetime(df['Datetime'])
    
    # Set 'Datetime' as the index
    df.set_index('Datetime', inplace=True)
        
    return df