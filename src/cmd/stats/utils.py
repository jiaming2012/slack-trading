import os
from polygon import RESTClient
import logging
import time
import requests
import json
import datetime
import pandas as pd
from typing import Optional

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def get_timespan_unit(u: str) -> int:
    if u == 'minute':
        unit_multiplier = 60
    elif u == 'hour':
        unit_multiplier = 3600
    elif u == 'day':
        unit_multiplier = 86400
    elif u == 'week':
        unit_multiplier = 604800
    else:
        raise Exception(f'Invalid timespan unit {u}')
    
    return unit_multiplier
        
class PolygonCandleResponse:
    def __init__(self):
        self.query_count = 0
        self.results_count = 0
        self.results = []
        self.next_url = None
        
def _make_request_url(symbol: str, timeframe_value: int, timeframe_unit: str, from_date: datetime, to_date: datetime) -> str:
    base_url = "https://api.polygon.io/v2/aggs/ticker"
    url = f"{base_url}/{symbol}/range/{timeframe_value}/{timeframe_unit}/{from_date.strftime('%Y-%m-%d')}/{to_date.strftime('%Y-%m-%d')}"
    return url

def _fetch_polygon_stock_chart(url: str, api_key: str) -> Optional[PolygonCandleResponse]:
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
    
def fetch_polygon_stock_chart_aggregated(symbol: str, timeframe_value: int, timeframe_unit: str, from_date: datetime, to_date: datetime, api_key = None) -> pd.DataFrame:
    rows = fetch_polygon_stock_chart_aggregated_as_list(symbol, timeframe_value, timeframe_unit, from_date, to_date, api_key)
    df = pd.DataFrame(rows)
    return df

def fetch_polygon_stock_chart_aggregated_as_list(symbol: str, timeframe_value: int, timeframe_unit: str, from_date: datetime, to_date: datetime, api_key: str) -> pd.DataFrame:
    # Fetch stock data
    if api_key is None:
        api_key = _fetch_api_key()
    
    back_off = [1, 2, 4, 8, 16, 32, 64, 128]
    aggregate_result = PolygonCandleResponse()

    counter = 0
    is_done = False

    while True:
        try:
            url = _make_request_url(symbol, timeframe_value, timeframe_unit, from_date, to_date)
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
            resp = _fetch_polygon_stock_chart(url, api_key)
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
        
    # Convert stock data to DataFrame
    exchange_tz = 'America/New_York'
    rows = []
    for a in aggregate_result.results:
        rows.append({
            'date': pd.to_datetime(a['t'], unit='ms').tz_localize('UTC').tz_convert(exchange_tz),
            'open': a['o'],
            'high': a['h'],
            'low': a['l'],
            'close': a['c'],
            'volume': a['v']
        })
        
    return rows

def _fetch_api_key() -> str:
    from dotenv import load_dotenv

    projectsDir = os.getenv('PROJECTS_DIR')
    if projectsDir is None:
        raise ValueError('PROJECTS_DIR environment variable is not set')
    
    secrets_dir = os.path.join(projectsDir, 'slack-trading', '.env.production-secrets')
    load_dotenv(secrets_dir)
    api_key = os.getenv('POLYGON_API_KEY')
    if api_key is None:
        raise ValueError('POLYGON_API_KEY environment variable is not set')
    
    return api_key

def build_polygon_client(api_key = None) -> RESTClient:
    if api_key is None:
        api_key = _fetch_api_key()
        
    client = RESTClient(api_key)
    return client