import requests
import json
from dataclasses import dataclass
from typing import List, Tuple
from enum import Enum

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

class SpreadType(Enum):
    VERTICAL_CALL = 'vertical_call'
    VERTICAL_PUT = 'vertical_put'

class SpreadDirection(Enum):
    LONG = 'long'
    SHORT = 'short'

@dataclass
class Spread:
    long_option: Option
    short_option: Option
    type: SpreadType
    direction: SpreadDirection

    def description(self) -> str:
        if self.direction == SpreadDirection.LONG:
            symbol = self.long_option.symbol
            expiration = self.long_option.expiration
            return f'{symbol} {expiration} {self.long_option.strike}/{self.short_option.strike} Call'
        elif self.direction == SpreadDirection.SHORT:
            symbol = self.long_option.symbol
            expiration = self.long_option.expiration
            return f'{symbol} {expiration} {self.short_option.strike}/{self.long_option.strike} Put'
        else:
            return f'{self.long_option.description}/{self.short_option.description}'
        
def filter_calls(options: List[Option]) -> List[Option]:
    return [option for option in options if option.option_type == 'call']

def filter_puts(options: List[Option]) -> List[Option]:
    return [option for option in options if option.option_type == 'put']

def sort_options_by_strike(options: List[Option]) -> List[Option]:
    return sorted(options, key=lambda option: option.strike)

def generate_short_vertical_spreads(options: List[Option]) -> List[Spread]:
    options = sort_options_by_strike(options)
    spreads = []
    for i in range(len(options)):
        for j in range(i + 1, len(options)):
            if options[i].option_type != options[j].option_type:
                continue

            if options[i].option_type == 'call':
                short_option = options[i]
                long_option = options[j]
                spread_type = SpreadType.VERTICAL_CALL
            else:
                short_option = options[j]
                long_option = options[i]
                spread_type = SpreadType.VERTICAL_PUT

            spreads.append(Spread(long_option, short_option, spread_type, SpreadDirection.SHORT))

    return spreads

def generate_long_vertical_spreads(options: List[Option]) -> List[Spread]:
    options = sort_options_by_strike(options)
    spreads = []
    for i in range(len(options)):
        for j in range(i + 1, len(options)):
            if options[i].option_type != options[j].option_type:
                continue

            if options[i].option_type == 'call':
                long_option = options[i]
                short_option = options[j]
                spread_type = SpreadType.VERTICAL_CALL
            else:
                long_option = options[j]
                short_option = options[i]
                spread_type = SpreadType.VERTICAL_PUT

            spreads.append(Spread(long_option, short_option, spread_type, SpreadDirection.LONG))

    return spreads

def fetch_options(symbol: str, expirationInDays: int, minDistance: int, maxStrikes: int) -> Tuple[Stock, List[Option]]:
    url = 'http://localhost:8080/options'
    response = requests.get(url, json={
        'symbol': symbol,
        'optionTypes': ['call', 'put'],
        'expirationsInDays': [expirationInDays],
        'minDistanceBetweenStrikes': minDistance,
        'maxNoOfStrikes': maxStrikes
    })

    response_payload = response.json()

    return Stock(**response_payload['stock']), [Option(**option) for option in response_payload['options']]

if __name__ == '__main__':
    result = fetch_options()
    print(result)