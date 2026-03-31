"""
Wheel Signals Datasource -- produces put sell signals from LTF supertrend data.

Extracts the put sell signal detection logic from WheelStrategy.check_for_put_signal()
into a stateless function. The caller provides a feature_vector_fn callable that
encapsulates the DataFrame access and supertrend lookback computation (same pattern
as covered_call_signals.py).

The Wheel strategy detects put sell opportunities during Phase 1 (SELL_PUTS) using
the same supertrend feature vector that drives covered call entries.

Usage (sim mode -- direct import)::

    from datasources.wheel_signals import produce_open_signals

    def my_feature_vector_fn(candle):
        # Returns (ltf_supertrend_count, ltf_supertrend_value, st_direction) or None
        ...

    signals = produce_open_signals(feature_vector_fn=my_feature_vector_fn, candle=bar)
    for sig in signals:
        print(sig["signal_name"], sig["ltf_supertrend_count"])
"""

from __future__ import annotations


def produce_open_signals(
    feature_vector_fn,
    candle,
    symbol: str = "",
) -> list:
    """Produce put sell signals from an LTF candle.

    Extracts the signal detection logic from WheelStrategy.check_for_put_signal()
    (wheel.py ~line 96). Uses the same supertrend feature vector extraction as
    the covered call datasource.

    The feature_vector_fn callable encapsulates the stateful supertrend lookback
    over the candles_ltf DataFrame. It takes a candle and returns either:
      - (ltf_supertrend_count, ltf_supertrend_value, st_direction) tuple, or
      - None if no supertrend direction change is detected.

    Args:
        feature_vector_fn: Callable(candle) -> tuple | None.
            Returns (ltf_supertrend_count, ltf_supertrend_value, st_direction)
            or None.
        candle: The current LTF candle (protobuf Bar or similar with .close,
                .datetime, .superT_50_3, .superD_50_3 attributes).
        symbol: Stock symbol string for the signal record.

    Returns:
        List of dicts with keys: signal_name, symbol, timestamp, price,
        ltf_supertrend_count, ltf_supertrend_value, ltf_supertrend_direction,
        expected_volatility.
        Empty list if no valid signal detected.
    """
    result = feature_vector_fn(candle)
    if result is None:
        return []

    ltf_supertrend_count, ltf_supertrend_value, st_direction = result

    return [{
        "signal_name": "SHORT_PUT_SIGNAL",
        "symbol": symbol,
        "timestamp": candle.datetime,
        "price": candle.close,
        "ltf_supertrend_count": ltf_supertrend_count,
        "ltf_supertrend_value": ltf_supertrend_value,
        "ltf_supertrend_direction": st_direction,
        "expected_volatility": 0.0,
    }]
