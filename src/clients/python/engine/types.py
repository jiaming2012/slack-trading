from dataclasses import dataclass
from enum import Enum
import pandas as pd


class RepositorySource(Enum):
    CSV = 'csv'
    POLYGON = 'polygon'

class OrderSide(Enum):
    BUY = 'buy'
    SELL = 'sell'
    SELL_SHORT = 'sell_short'
    BUY_TO_COVER = 'buy_to_cover'
    BUY_TO_OPEN = 'buy_to_open'
    SELL_TO_CLOSE = 'sell_to_close'
    SELL_TO_OPEN = 'sell_to_open'
    BUY_TO_CLOSE = 'buy_to_close'

class LiveAccountType(Enum):
    MARGIN = 'margin'
    PAPER = 'paper'

class OpenSignalName(Enum):
    CROSS_ABOVE_20 = 1
    CROSS_BELOW_80 = 2
    SUPERTREND_STACK_SIGNAL = 3
    LONG_OPTION_ENTRY = 4

@dataclass
class OpenSignal:
    name: OpenSignalName
    timestamp: pd.Timestamp
    max_price_prediction: float
    min_price_prediction: float

@dataclass
class OpenSignalV2:
    name: OpenSignalName
    symbol: str
    timestamp: pd.Timestamp
    max_price_prediction: float
    min_price_prediction: float
    additional_equity_risk: float
    max_price_prediction_r2: float
    max_price_prediction_mse: float
    max_price_prediction_n: int
    min_price_prediction_r2: float
    min_price_prediction_mse: float
    min_price_prediction_n: int

@dataclass
class OpenSignalV3:
    name: OpenSignalName
    symbol: str
    timestamp: pd.Timestamp
    kwargs: dict
    additional_equity_risk: float
