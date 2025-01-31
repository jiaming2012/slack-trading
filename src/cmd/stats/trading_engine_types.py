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
    max_price_prediction_std_dev: float
    min_price_prediction_std_dev: float
    sl_shift: float
    tp_shift: float