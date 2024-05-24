import requests
import json
from dataclasses import dataclass
from typing import List, Tuple

@dataclass
class Stock:
    bid: float
    ask: float

@dataclass
class Option:
    symbol: str
    underlying_symbol: str
    description: str
    strike: float
    option_type: str
    contract_size: int
    expiration: str
    expiration_type: str
    bid: float
    ask: float

def fetch_options() -> Tuple[Stock, List[Option]]:
    url = 'http://localhost:8085/options'
    response = requests.get(url, json={
        'symbol': 'coin',
        'optionTypes': ['call', 'put'],
        'expirationsInDays': [7],
        'minDistanceBetweenStrikes': 10,
        'maxNoOfStrikes': 3
    })

    response_payload = response.json()

    return Stock(**response_payload['stock']), [Option(**option) for option in response_payload['options']]

if __name__ == '__main__':
    result = fetch_options()
    print(result)