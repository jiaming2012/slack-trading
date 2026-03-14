"""
Pure-Python candlestick pattern detection from OHLC values.

No DataFrame or pandas dependency — operates on individual float values
so patterns are easy to unit test with contrived data.
"""

from typing import Tuple, Optional


def _body(open: float, close: float) -> float:
    """Absolute size of the candle body."""
    return abs(close - open)


def _range(high: float, low: float) -> float:
    """Full candle range (high - low)."""
    return high - low


def _upper_wick(open: float, high: float, close: float) -> float:
    return high - max(open, close)


def _lower_wick(open: float, low: float, close: float) -> float:
    return min(open, close) - low


def is_pin_bar(
    open: float, high: float, low: float, close: float,
    wick_ratio: float = 2.0,
) -> Tuple[bool, Optional[str]]:
    """
    Detect a pin bar (aka pinocchio bar).

    Bearish pin bar: long upper wick, small body near the low.
    Bullish pin bar: long lower wick, small body near the high.

    Parameters
    ----------
    wick_ratio : float
        Minimum ratio of the dominant wick to the body size.

    Returns
    -------
    (is_pin_bar, direction)
        direction is "bullish", "bearish", or None.
    """
    body = _body(open, close)
    candle_range = _range(high, low)
    if candle_range == 0:
        return False, None

    upper = _upper_wick(open, high, close)
    lower = _lower_wick(open, low, close)

    # Avoid division by zero for doji-like candles: use range fraction instead
    if body == 0:
        body = candle_range * 0.01  # treat as very small body

    # Bearish pin bar: long upper wick dominates
    if upper >= body * wick_ratio and upper > lower * 2:
        return True, "bearish"

    # Bullish pin bar: long lower wick dominates
    if lower >= body * wick_ratio and lower > upper * 2:
        return True, "bullish"

    return False, None


def is_engulfing(
    prev_open: float, prev_close: float,
    curr_open: float, curr_close: float,
) -> Tuple[bool, Optional[str]]:
    """
    Detect a bullish or bearish engulfing pattern.

    Bullish engulfing: previous candle is red, current candle is green
    and its body fully engulfs the previous body.

    Bearish engulfing: previous candle is green, current candle is red
    and its body fully engulfs the previous body.

    Returns
    -------
    (is_engulfing, direction)
        direction is "bullish", "bearish", or None.
    """
    prev_body = _body(prev_open, prev_close)
    curr_body = _body(curr_open, curr_close)

    if prev_body == 0 or curr_body == 0:
        return False, None

    prev_is_red = prev_close < prev_open
    prev_is_green = prev_close > prev_open
    curr_is_red = curr_close < curr_open
    curr_is_green = curr_close > curr_open

    # Bullish engulfing: prev red, curr green, curr body engulfs prev body
    if prev_is_red and curr_is_green:
        if curr_open <= prev_close and curr_close >= prev_open:
            return True, "bullish"

    # Bearish engulfing: prev green, curr red, curr body engulfs prev body
    if prev_is_green and curr_is_red:
        if curr_open >= prev_close and curr_close <= prev_open:
            return True, "bearish"

    return False, None


def is_hammer(
    open: float, high: float, low: float, close: float,
    lower_wick_ratio: float = 2.0,
    upper_wick_max_ratio: float = 0.3,
) -> bool:
    """
    Detect a hammer candle.

    A hammer has a small body near the top of the range with a long lower
    shadow (at least ``lower_wick_ratio`` times the body) and a very short
    upper shadow (at most ``upper_wick_max_ratio`` times the body).
    """
    body = _body(open, close)
    candle_range = _range(high, low)
    if candle_range == 0:
        return False

    if body == 0:
        body = candle_range * 0.01

    lower = _lower_wick(open, low, close)
    upper = _upper_wick(open, high, close)

    return lower >= body * lower_wick_ratio and upper <= body * upper_wick_max_ratio


def is_doji(
    open: float, close: float, high: float, low: float,
    threshold: float = 0.001,
) -> bool:
    """
    Detect a doji candle.

    A doji has a body smaller than ``threshold`` fraction of the full range.
    """
    candle_range = _range(high, low)
    if candle_range == 0:
        return False

    body = _body(open, close)
    return (body / candle_range) < threshold
