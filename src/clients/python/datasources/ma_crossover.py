"""
MA Crossover Datasource — produces trade signals from HTF bar data.

Extracts the signal detection logic from MeanReversionStrategy._process_htf_candle()
(lines 241-257) into a stateless, reusable function. This is the first datasource
module in the TradeSignal framework migration.

The caller is responsible for maintaining prev_bar state between calls.

Usage (sim mode — direct import)::

    from datasources.ma_crossover import produce_signals

    signals = produce_signals(bar_dict, prev_bar, pdf)
    for sig in signals:
        print(sig["signal_key"], sig["pdf_entry"])

Note: Live/RPC mode (WriteSignal endpoint) is deferred to Phase 19.
"""

from __future__ import annotations

from lib.pdf_builder import detect_atomic_signals_on_bar


def produce_signals(bar_dict: dict, prev_bar: dict | None, pdf) -> list:
    """Produce MA crossover trade signals from an HTF bar.

    Extracts the signal detection logic from MeanReversionStrategy._process_htf_candle()
    lines 241-257. Returns a list of signal dicts (empty if no valid signal).

    Args:
        bar_dict: Current HTF bar as dict (from _bar_to_dict).
        prev_bar: Previous HTF bar (needed by detect_atomic_signals_on_bar for
                  crossover detection). MUST be the bar from the PREVIOUS call,
                  NOT the current bar.
        pdf: PDFDocument instance for signal lookup.

    Returns:
        List of dicts with keys: signal_key, bar_dict, pdf_entry.
        Empty list if no valid signal detected.
    """
    signals = detect_atomic_signals_on_bar(bar_dict, prev_bar, timeframe="ltf")

    if not signals or not pdf:
        return []

    key = "|".join(sorted(signals))
    pdf_entry = pdf.get_signal(key)

    if not pdf_entry or not pdf_entry.sufficient_samples:
        return []

    return [{
        "signal_key": key,
        "bar_dict": bar_dict,
        "pdf_entry": pdf_entry,
    }]
