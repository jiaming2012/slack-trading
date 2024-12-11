import argparse
import requests
from typing import Optional, Dict, Any
import logging
import json
from polygon import RESTClient
from dotenv import load_dotenv
import pandas as pd
import os
import time
import datetime
import plotly.graph_objects as go
from plotly.subplots import make_subplots


logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class PolygonCandleResponse:
    def __init__(self):
        self.query_count = 0
        self.results_count = 0
        self.results = []
        self.next_url = None

def make_request_url(symbol: str, timeframe_value: int, timeframe_unit: str, from_date: datetime, to_date: datetime) -> str:
    base_url = "https://api.polygon.io/v2/aggs/ticker"
    url = f"{base_url}/{symbol}/range/{timeframe_value}/{timeframe_unit}/{from_date.strftime('%Y-%m-%d')}/{to_date.strftime('%Y-%m-%d')}"
    return url

def fetch_polygon_stock_chart(url: str, api_key: str) -> Optional[PolygonCandleResponse]:
    try:
        params = {
            'sort': 'asc',
            'adjusted': 'false',
            'apiKey': api_key
        }
        headers = {
            'Accept': 'application/json'
        }

        logger.info(f"fetching from {url}")

        response = requests.get(url, params=params, headers=headers, timeout=10)
        response.raise_for_status()

        dto = response.json()
        result = PolygonCandleResponse()
        result.query_count = dto.get('queryCount', 0)
        result.results_count = dto.get('resultsCount', 0)
        result.results = dto.get('results', [])
        result.next_url = dto.get('next_url')

        return result

    except requests.RequestException as e:
        logger.error(f"fetchPolygonStockChart: failed to fetch stock tick: {e}")
        return None
    except json.JSONDecodeError as e:
        logger.error(f"fetchPolygonStockChart: failed to decode json: {e}")
        return None

def fetch_polygon_stock_chart_aggregated(symbol: str, timeframe_value: int, timeframe_unit: str, from_date: datetime, to_date: datetime, api_key: str) -> Optional[PolygonCandleResponse]:
    back_off = [1, 2, 4, 8, 16, 32, 64, 128]
    aggregate_result = PolygonCandleResponse()

    counter = 0
    is_done = False

    while True:
        try:
            url = make_request_url(symbol, timeframe_value, timeframe_unit, from_date, to_date)
        except Exception as e:
            logger.error(f"FetchPolygonStockChart: failed to make request URL: {e}")
            return None

        aggregate_result = PolygonCandleResponse()

        if counter > 0:
            logger.warning(f"FetchPolygonStockChart: backoff {back_off[counter]} seconds")
            time.sleep(back_off[counter])

        if counter < len(back_off) - 1:
            counter += 1

        while True:
            resp = fetch_polygon_stock_chart(url, api_key)
            if resp is None:
                logger.error("FetchPolygonStockChart: failed to fetch stock chart")
                break

            aggregate_result.query_count += resp.query_count
            aggregate_result.results_count += resp.results_count
            aggregate_result.results.extend(resp.results)

            if resp.next_url is None:
                is_done = True
                break

            url = resp.next_url
            time.sleep(0.05)

        if len(aggregate_result.results) == 0:
            logger.error("FetchPolygonStockChart: no results found")
            return None

        if is_done:
            break

    return aggregate_result
    
def fetch_playground(id: str, host: str) -> dict:
    url = f"{host}/playground/{id}/account"
    response = requests.get(url)
    response.raise_for_status()
    return response.json()

def fetch_api_key() -> str:
    projectsDir = os.getenv('PROJECTS_DIR')
    if projectsDir is None:
        raise ValueError('PROJECTS_DIR environment variable is not set')
    
    secrets_dir = os.path.join(projectsDir, 'slack-trading', '.env.production-secrets')
    load_dotenv(secrets_dir)
    api_key = os.getenv('POLYGON_API_KEY')
    if api_key is None:
        raise ValueError('POLYGON_API_KEY environment variable is not set')
    
    return api_key

def build_polygon_client() -> RESTClient:
    api_key = fetch_api_key()
    client = RESTClient(api_key)
    return client

def get_meta(account: dict) -> dict:
    return account['meta']

def get_polygon_date(date_in_rtf_3339: str) -> datetime:
    date_stamp = date_in_rtf_3339[:date_in_rtf_3339.find('T')]
    return datetime.datetime.strptime(date_stamp, '%Y-%m-%d')

parser = argparse.ArgumentParser(description="The script will plot trades found in a playground.")
parser.add_argument('--playground-id', type=str, help='The id of the playground to plot', required=True)
parser.add_argument('--host', type=str, help='The host of the playground', default='http://localhost:8080')
args = parser.parse_args()

playground = fetch_playground(args.playground_id, args.host)
# polygon_client = build_polygon_client()
api_key = fetch_api_key()

meta = get_meta(playground)

if len(meta['symbols']) > 1:
    raise ValueError('Only one symbol is supported')

# Polygon input parameters
symbol = meta['symbols'][0]
timeframe_value = 1
timeframe_unit = 'minute'
from_date = get_polygon_date(meta['start_date'])
to_date = get_polygon_date(meta['end_date'])

# Fetch data
data = fetch_polygon_stock_chart_aggregated(symbol, timeframe_value, timeframe_unit, from_date, to_date, api_key)

# Convert to DataFrame
rows = []
for a in data.results:
    df = rows.append({
        'Date': pd.to_datetime(a['t'], unit='ms'),
        'Open': a['o'],
        'High': a['h'],
        'Low': a['l'],
        'Close': a['c'],
        'Volume': a['v']
    })
    
df = pd.DataFrame(rows)
    
print(f'df: {df}')
print(f'Min Volume: {df["Volume"].min()}')
print(f'Max Volume: {df["Volume"].max()}')

# Create subplots
fig = make_subplots(rows=1, cols=1, shared_xaxes=True,
                    subplot_titles=(symbol,))

# Add candlestick chart
fig.add_trace(go.Candlestick(
    x=df['Date'],
    open=df['Open'],
    high=df['High'],
    low=df['Low'],
    close=df['Close'],
    increasing_line_color='green',
    decreasing_line_color='red',
    name='Candle'
), row=1, col=1)

fig.update_layout(
    title=f'Playground ID {args.playground_id}',
    yaxis_title='Price',
    xaxis_title='Date',
    xaxis_rangeslider_visible=False,
    xaxis=dict(
        tickformat='%Y-%m-%d %H:%M',
        tickangle=45,
        tickmode='auto'
    ),
    yaxis=dict(
        autorange=True,
    )
)

# Add volume bar chart
# fig.add_trace(go.Bar(
#     x=df['Date'],
#     y=df['Volume'],
#     name='Volume',
#     yaxis='y2'
# ), row=2, col=1)

# Update layout to include secondary y-axis
fig.update_layout(
    yaxis2=dict(
        title='Volume',
        overlaying='y',
        side='right'
    )
)

# Update x-axis and y-axis to scale
fig.update_xaxes(type='category', row=1, col=1)
fig.update_yaxes(row=1, col=1, autorange=True)
# fig.update_yaxes(type='linear', row=2, col=1)

# Show the plot
fig.show()