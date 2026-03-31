"""
Credit Spread Signals Datasource -- produces trade signals from HTF bar data.

Extracts the signal detection logic from CreditSpreadStrategy._process_htf_candle()
(lines 495-516) into a stateless, reusable function. This datasource follows the
same pattern as ma_crossover.py but adds direction and horizon data needed by the
credit spread strategy (which trades both bullish and bearish signals).

The caller is responsible for maintaining prev_bar state between calls.

Usage (sim mode -- direct import)::

    from datasources.credit_spread_signals import produce_signals

    signals = produce_signals(bar_dict, prev_bar, pdf, htf_horizon="1h")
    for sig in signals:
        print(sig["signal_key"], sig["direction"], sig["pdf_entry"])
"""

from __future__ import annotations

from lib.pdf_builder import detect_atomic_signals_on_bar


def produce_signals(
    bar_dict: dict,
    prev_bar: dict | None,
    pdf,
    htf_horizon: str = "1h",
) -> list:
    """Produce credit spread trade signals from an HTF bar.

    Extracts the signal detection logic from CreditSpreadStrategy._process_htf_candle()
    lines 495-516. Returns a list of signal dicts (empty if no valid signal).

    Args:
        bar_dict: Current HTF bar as dict (from _bar_to_dict).
        prev_bar: Previous HTF bar (needed by detect_atomic_signals_on_bar for
                  crossover detection). MUST be the bar from the PREVIOUS call,
                  NOT the current bar.
        pdf: PDFDocument instance for signal lookup.
        htf_horizon: Horizon key to look up in the PDF entry (default "1h").

    Returns:
        List of dicts with keys: signal_key, bar_dict, pdf_entry, direction, horizon.
        Empty list if no valid signal detected.
    """
    signals = detect_atomic_signals_on_bar(bar_dict, prev_bar, timeframe="ltf")

    if not signals or not pdf:
        return []

    key = "|".join(sorted(signals))
    pdf_entry = pdf.get_signal(key)

    if not pdf_entry or not pdf_entry.sufficient_samples:
        return []

    horizon = pdf_entry.horizons.get(htf_horizon)
    if not horizon:
        return []

    mean = horizon.mean

    if mean > 0:
        direction = "bullish"
    elif mean < 0:
        direction = "bearish"
    else:
        return []

    return [{
        "signal_key": key,
        "bar_dict": bar_dict,
        "pdf_entry": pdf_entry,
        "direction": direction,
        "horizon": horizon,
    }]
