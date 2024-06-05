import requests
import json
import uuid
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
    Underlying: str
    long_option: Option
    short_option: Option
    type: SpreadType
    direction: SpreadDirection

    def description(self) -> str:
        side = 'Call' if self.type == SpreadType.VERTICAL_CALL else ('Put' if self.type == SpreadType.VERTICAL_PUT else 'Unknown')
        
        if self.direction == SpreadDirection.LONG:
            expiration = self.long_option.expiration
            return f'{self.Underlying.upper()} {self.long_option.strike}/{self.short_option.strike} {side} {expiration}'
        elif self.direction == SpreadDirection.SHORT:
            expiration = self.long_option.expiration
            return f'{self.Underlying.upper()} {self.short_option.strike}/{self.long_option.strike} {side} {expiration}'
        else:
            return f'{self.long_option.description}/{self.short_option.description}'
        
def filter_calls(options: List[Option]) -> List[Option]:
    return [option for option in options if option.option_type == 'call']

def filter_puts(options: List[Option]) -> List[Option]:
    return [option for option in options if option.option_type == 'put']

def sort_options_by_strike(options: List[Option]) -> List[Option]:
    return sorted(options, key=lambda option: option.strike)

def generate_short_vertical_spreads(options: List[Option], underlyingSymbol: str) -> List[Spread]:
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

            spreads.append(Spread(underlyingSymbol, long_option, short_option, spread_type, SpreadDirection.SHORT))

    return spreads

def generate_long_vertical_spreads(options: List[Option], underlyingSymbol: str) -> List[Spread]:
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

            spreads.append(Spread(underlyingSymbol, long_option, short_option, spread_type, SpreadDirection.LONG))

    return spreads

def fetch_options(url: str, symbol: str, expirationInDays: int, minDistance: int, maxStrikes: int) -> Tuple[Stock, List[Option]]:
    url = url
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
    stock, options = fetch_options('http://localhost:8080/options', 'SPX', 0, 10, 5)

    result = {
        'stock': stock.__dict__,
        'options': [option.__dict__ for option in options]
    }

    # write to a unique tmp file
    outDir = f'tmp-{uuid.uuid4()}.json'
    with open(outDir, 'w') as file:
        json.dump(result, file)

    output = {'output': {'outDir': outDir}}

    print(json.dumps(output))