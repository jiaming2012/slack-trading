"""
PDF Wheel Signals Datasource -- produces compound trade signals from LTF bar data.

Extracts the signal detection logic from PDFWheelStrategy.check_for_pdf_put_signals()
into a stateless function. This datasource detects atomic signals via
detect_atomic_signals_on_bar(), merges them with daily context signals, builds a
compound key, and looks up the PDF for a matching distribution.

Unlike the simple covered_call_signals/wheel_signals datasources (which use a
supertrend feature vector), PDF wheel signals use compound signal detection with
PDF-based validation -- the same pattern as credit_spread_signals.py but with
subset matching via PDFDocument.find_best_signal().

The caller is responsible for maintaining prev_bar and daily_signals state.

Usage (sim mode -- direct import)::

    from datasources.pdf_wheel_signals import produce_signals

    signals = produce_signals(bar_dict, prev_bar, pdf, daily_signals, ci_threshold=0.01)
    for sig in signals:
        print(sig["compound_key"], sig["pdf_entry"])
"""

from __future__ import annotations

from typing import Dict, List, Optional

from lib.pdf_builder import detect_atomic_signals_on_bar, _get, _get_dt


def produce_signals(
    bar_dict: dict,
    prev_bar: dict | None,
    pdf,
    daily_signals: Dict[str, List[str]] | None = None,
    ci_threshold: float = 0.01,
) -> list:
    """Produce PDF-guided compound trade signals from an LTF bar.

    Extracts the signal detection logic from PDFWheelStrategy.check_for_pdf_put_signals()
    (pdf_wheel.py ~line 495). Returns a list of signal dicts (empty if no valid signal).

    Args:
        bar_dict: Current LTF bar as dict (from _bar_to_dict).
        prev_bar: Previous LTF bar (needed by detect_atomic_signals_on_bar for
                  crossover detection). MUST be the bar from the PREVIOUS call.
        pdf: PDFDocument instance for signal lookup (must have find_best_signal).
        daily_signals: Dict mapping date strings ("YYYY-MM-DD") to lists of
                       daily atomic signal names. Used for compound key construction.
        ci_threshold: Maximum 95% CI width for a signal to be considered usable
                      (default 0.01).

    Returns:
        List of dicts with keys: compound_key, bar_dict, pdf_entry, price, timestamp,
        signal_name.
        Empty list if no valid signal detected.
    """
    if daily_signals is None:
        daily_signals = {}

    ltf_sigs = detect_atomic_signals_on_bar(bar_dict, prev_bar=prev_bar, timeframe="ltf")

    if not ltf_sigs:
        return []

    dt = _get_dt(bar_dict)
    if dt is None:
        return []

    if not pdf:
        return []

    date_key = dt.strftime("%Y-%m-%d")
    daily_sigs = daily_signals.get(date_key, [])
    all_components = sorted(set(ltf_sigs + daily_sigs))

    # Try exact match first, then subsets down to single signals
    result = pdf.find_best_signal(
        all_components,
        min_ci_width=ci_threshold,
        min_components=1,
    )
    if result is None:
        return []

    matched_key, pdf_entry = result
    price = float(_get(bar_dict, "close"))

    return [{
        "compound_key": matched_key,
        "bar_dict": bar_dict,
        "pdf_entry": pdf_entry,
        "price": price,
        "timestamp": dt,
        "signal_name": "PDF_SHORT_PUT_SIGNAL",
    }]
