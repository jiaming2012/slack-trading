"""
Options MA Crossover Datasource -- produces trade signals from HTF bar data.

Extracts the signal detection logic from OptionsMeanReversionStrategy._process_htf_candle()
(lines 234-249) into a stateless, reusable function. This datasource extends the Phase 21
ma_crossover pattern with direction (bullish/bearish) and horizon_key support needed by
options strategies.

The caller is responsible for maintaining prev_bar state between calls.

Usage (sim mode -- direct import)::

    from datasources.options_ma_crossover import produce_signals

    signals = produce_signals(bar_dict, prev_bar, pdf, "1h")
    for sig in signals:
        print(sig["signal_key"], sig["direction"], sig["pdf_entry"])

Note: Live/RPC mode (WriteSignal endpoint) is deferred to Phase 19+.
"""

from __future__ import annotations

from lib.pdf_builder import detect_atomic_signals_on_bar


def produce_signals(
    bar_dict: dict, prev_bar: dict | None, pdf, horizon_key: str = "1h",
) -> list:
    """Produce options MA crossover trade signals from an HTF bar.

    Extracts the signal detection logic from
    OptionsMeanReversionStrategy._process_htf_candle() lines 234-249.
    Returns a list of signal dicts (empty if no valid signal).

    Unlike ma_crossover.produce_signals(), this also checks the horizon mean
    direction and returns bullish/bearish classification.

    Args:
        bar_dict: Current HTF bar as dict (from _bar_to_dict).
        prev_bar: Previous HTF bar (needed by detect_atomic_signals_on_bar for
                  crossover detection). MUST be the bar from the PREVIOUS call,
                  NOT the current bar.
        pdf: PDFDocument instance for signal lookup.
        horizon_key: Which horizon to check in the PDF entry (default "1h").

    Returns:
        List of dicts with keys: signal_key, bar_dict, pdf_entry, direction,
        horizon_key.  Empty list if no valid signal detected.
    """
    signals = detect_atomic_signals_on_bar(bar_dict, prev_bar, timeframe="ltf")

    if not signals or not pdf:
        return []

    key = "|".join(sorted(signals))
    pdf_entry = pdf.get_signal(key)

    if not pdf_entry or not pdf_entry.sufficient_samples:
        return []

    horizon = pdf_entry.horizons.get(horizon_key)
    if not horizon:
        return []

    result = []
    if horizon.mean > 0:
        result.append({
            "signal_key": key,
            "bar_dict": bar_dict,
            "pdf_entry": pdf_entry,
            "direction": "bullish",
            "horizon_key": horizon_key,
        })
    elif horizon.mean < 0:
        result.append({
            "signal_key": key,
            "bar_dict": bar_dict,
            "pdf_entry": pdf_entry,
            "direction": "bearish",
            "horizon_key": horizon_key,
        })

    return result
