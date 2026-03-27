from dataclasses import dataclass
from enum import Enum
from typing import Any, Dict, Optional

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


@dataclass
class SignalDecision:
    """Structured signal decision record for strategy telemetry.

    Captures every decision a strategy makes (place or skip) with
    the reason, enabling post-hoc analysis of "why was this trade
    placed or not placed?"

    Fields per D-03: signal_type, direction, decision, reason,
    symbol, playground_id, trace_id.
    Full indicator dump (D-04) gated by STRATEGY_LOG_VERBOSE env var.
    """
    signal_type: str          # e.g. "covered_call", "mean_reversion_dip"
    direction: str            # "long", "short", "neutral"
    decision: str             # "place" or "skip"
    reason: str               # e.g. "below threshold", "position full", "signal triggered"
    symbol: str
    playground_id: str
    trace_id: str = ""
    indicators: Optional[Dict[str, Any]] = None  # full dump when verbose (D-04)
