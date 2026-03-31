"""
Covered Call Signals Datasource -- produces open signals from LTF supertrend data.

Extracts the signal detection logic from OptionsStrategyBasic.check_for_new_signal()
and _get_feature_vector() into a stateless function. The caller provides a
feature_vector_fn callable that encapsulates the DataFrame access and supertrend
lookback computation.

This follows the callable-based datasource pattern from Phase 22-03:
the datasource is pure logic, and the strategy supplies the state-dependent
feature vector extraction.

Usage (sim mode -- direct import)::

    from datasources.covered_call_signals import produce_open_signals

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
    """Produce covered call open signals from an LTF candle.

    Extracts the signal detection logic from OptionsStrategyBasic.check_for_new_signal()
    (covered_call.py ~line 340) and _get_feature_vector() (~line 326).

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
        "signal_name": "SHORT_CALL_SIGNAL",
        "symbol": symbol,
        "timestamp": candle.datetime,
        "price": candle.close,
        "ltf_supertrend_count": ltf_supertrend_count,
        "ltf_supertrend_value": ltf_supertrend_value,
        "ltf_supertrend_direction": st_direction,
        "expected_volatility": 0.0,
    }]
