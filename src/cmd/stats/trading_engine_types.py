from dataclasses import dataclass
from enum import Enum
import pandas as pd

class OpenSignalName(Enum):
    CROSS_ABOVE_20 = 1
    CROSS_BELOW_80 = 2

@dataclass
class OpenSignal:
    name: OpenSignalName
    date: pd.Timestamp
    max_price_prediction: float
    min_price_prediction: float
    
@dataclass
class OpenSignalV2:
    name: OpenSignalName
    date: pd.Timestamp
    max_price_prediction: float
    min_price_prediction: float
    additional_equity_risk: float
    max_price_prediction_r2: float
    max_price_prediction_mse: float
    max_price_prediction_n: int
    min_price_prediction_r2: float
    min_price_prediction_mse: float
    min_price_prediction_n: int