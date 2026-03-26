"""
Offline PDF builder: scan historical bars, detect compound signals,
compute forward-return distributions, and produce a PDFDocument.

Usage (standalone)::

    from lib.pdf_builder import PDFBuilder

    builder = PDFBuilder(symbol="AAPL", ltf_bars=ltf_bars, daily_bars=daily_bars)
    pdf = builder.build()
    pdf.save("aapl_pdf.json")

``ltf_bars`` and ``daily_bars`` are lists of bar-like objects (dicts or
attribute-bearing objects) with at least these fields:

    open, high, low, close, datetime,
    superD_50_3, stochrsi_k_14_14_3_3,
    stochrsi_cross_above_20, stochrsi_cross_below_80,
    cdl_hammer, cdl_doji_10_0_1,
    sma_50, sma_100, sma_200
"""

from __future__ import annotations

import math
from collections import defaultdict
from datetime import datetime
from typing import Any, Dict, List, Optional, Tuple

from lib.candlestick_patterns import is_pin_bar, is_engulfing, is_hammer, is_doji
from lib.pdf_types import (
    ATOMIC_SIGNAL_CATALOG,
    DEFAULT_HORIZONS,
    CompoundSignal,
    HorizonStats,
    PDFDocument,
    SignalPDF,
    ci_95_width,
    compute_percentiles,
)


# ------------------------------------------------------------------ #
# Helpers to access bar fields uniformly (dict or object)
# ------------------------------------------------------------------ #

def _get(bar: Any, field: str, default: Any = 0.0) -> Any:
    """Access a field from a bar that may be a dict or an object."""
    if isinstance(bar, dict):
        return bar.get(field, default)
    return getattr(bar, field, default)


def _get_dt(bar: Any) -> Optional[datetime]:
    """Return the bar datetime as a datetime object (or None)."""
    val = _get(bar, "datetime", None)
    if val is None:
        return None
    if isinstance(val, datetime):
        return val
    if isinstance(val, str):
        # Accept common ISO formats
        for fmt in ("%Y-%m-%dT%H:%M:%S%z", "%Y-%m-%dT%H:%M:%S", "%Y-%m-%d %H:%M:%S"):
            try:
                return datetime.strptime(val, fmt)
            except ValueError:
                continue
    return None


# ------------------------------------------------------------------ #
# Atomic signal detection
# ------------------------------------------------------------------ #

def detect_atomic_signals_on_bar(
    bar: Any,
    prev_bar: Any = None,
    timeframe: str = "ltf",
) -> List[str]:
    """
    Return a list of atomic signal names present on *bar*.

    Parameters
    ----------
    bar : dict-like or object
        Current bar with OHLC + indicator fields.
    prev_bar : dict-like or object, optional
        Previous bar (needed for engulfing, supertrend flip, SMA cross).
    timeframe : str
        "ltf" or "daily".  Daily bars emit ``daily_supertrend_up/down``
        instead of ``supertrend_flip_up/down``.
    """
    o = float(_get(bar, "open"))
    h = float(_get(bar, "high"))
    lo = float(_get(bar, "low"))
    c = float(_get(bar, "close"))

    signals: List[str] = []

    # --- Candlestick patterns (OHLC-based) ---
    is_pb, pb_dir = is_pin_bar(o, h, lo, c)
    if is_pb:
        signals.append(f"{pb_dir}_pin_bar")

    if prev_bar is not None:
        po = float(_get(prev_bar, "open"))
        pc = float(_get(prev_bar, "close"))
        is_eng, eng_dir = is_engulfing(po, pc, o, c)
        if is_eng:
            signals.append(f"engulfing_{eng_dir}")

    if is_hammer(o, h, lo, c):
        signals.append("hammer")

    if is_doji(o, c, h, lo):
        signals.append("doji")

    # --- StochRSI ---
    stoch_k = float(_get(bar, "stochrsi_k_14_14_3_3", 50))
    if stoch_k > 80:
        signals.append("stochrsi_above_80")
    if stoch_k < 20:
        signals.append("stochrsi_below_20")

    # Proto bool crossover fields
    if _get(bar, "stochrsi_cross_above_20", False):
        signals.append("stochrsi_cross_above_20")
    if _get(bar, "stochrsi_cross_below_80", False):
        signals.append("stochrsi_cross_below_80")

    # --- SuperTrend ---
    if timeframe == "daily":
        super_d = int(_get(bar, "superD_50_3", 0))
        if super_d == 1:
            signals.append("daily_supertrend_up")
        elif super_d == -1:
            signals.append("daily_supertrend_down")
    else:
        # Detect flip (requires prev_bar)
        if prev_bar is not None:
            curr_d = int(_get(bar, "superD_50_3", 0))
            prev_d = int(_get(prev_bar, "superD_50_3", 0))
            if prev_d == -1 and curr_d == 1:
                signals.append("supertrend_flip_up")
            elif prev_d == 1 and curr_d == -1:
                signals.append("supertrend_flip_down")

    # --- SMA crossovers ---
    if prev_bar is not None:
        for sma_field, sma_len in [("sma_50", 50), ("sma_100", 100), ("sma_200", 200)]:
            sma_val = float(_get(bar, sma_field, 0))
            prev_close = float(_get(prev_bar, "close", 0))
            prev_sma = float(_get(prev_bar, sma_field, 0))
            if sma_val > 0 and prev_sma > 0:
                if prev_close <= prev_sma and c > sma_val:
                    signals.append(f"sma_{sma_len}_cross_above")
                elif prev_close >= prev_sma and c < sma_val:
                    signals.append(f"sma_{sma_len}_cross_below")

    return signals


# ------------------------------------------------------------------ #
# Builder
# ------------------------------------------------------------------ #

class PDFBuilder:
    """
    Scan historical bars, detect compound signals, measure forward returns,
    and produce a :class:`PDFDocument`.

    Parameters
    ----------
    symbol : str
        Stock ticker (e.g. "AAPL").
    ltf_bars : list
        Lower-timeframe bars (e.g. 15-min) with OHLC + indicator fields.
    daily_bars : list
        Daily bars with OHLC + indicator fields.
    ltf_period_seconds : int
        Period of the lower timeframe in seconds (default 900 = 15 min).
    horizons : dict[str, int] | None
        Mapping of horizon name to number of LTF bars forward.
    min_ci_width : float
        Maximum 95 % CI width for a signal to be considered "sufficient".
    """

    def __init__(
        self,
        symbol: str,
        ltf_bars: list,
        daily_bars: list,
        ltf_period_seconds: int = 900,
        horizons: Optional[Dict[str, int]] = None,
        min_ci_width: float = 0.005,
        htf_timeframe: str = "daily",
        return_model: str = "empirical",
    ):
        self.symbol = symbol
        self.ltf_bars = ltf_bars
        self.daily_bars = daily_bars
        self.ltf_period_seconds = ltf_period_seconds
        self.horizons = horizons or dict(DEFAULT_HORIZONS)
        self.min_ci_width = min_ci_width
        self.htf_timeframe = htf_timeframe
        self.return_model = return_model

    # ---- daily context lookup --------------------------------------- #

    def _build_daily_context(self) -> Dict[str, List[str]]:
        """
        For each calendar date, return the list of atomic signals detected
        on the daily bar for that date.

        Returns a dict mapping date string (YYYY-MM-DD) to signal list.
        """
        ctx: Dict[str, List[str]] = {}
        prev = None
        for bar in self.daily_bars:
            dt = _get_dt(bar)
            if dt is None:
                prev = bar
                continue
            signals = detect_atomic_signals_on_bar(bar, prev_bar=prev, timeframe=self.htf_timeframe)
            ctx[dt.strftime("%Y-%m-%d")] = signals
            prev = bar
        return ctx

    # ---- compound signal construction ------------------------------- #

    def _detect_ltf_signals(self) -> List[Tuple[int, List[str]]]:
        """
        Scan all LTF bars and return (bar_index, [atomic_signal_names])
        for bars where at least one signal fires.
        """
        results: List[Tuple[int, List[str]]] = []
        prev = None
        for i, bar in enumerate(self.ltf_bars):
            sigs = detect_atomic_signals_on_bar(bar, prev_bar=prev, timeframe="ltf")
            if sigs:
                results.append((i, sigs))
            prev = bar
        return results

    def build_compound_signals(
        self,
        ltf_signals: List[Tuple[int, List[str]]],
        daily_context: Dict[str, List[str]],
    ) -> List[CompoundSignal]:
        """
        Combine LTF atomic signals with daily context for the same
        trading day to form compound signals.
        """
        compounds: List[CompoundSignal] = []
        for idx, ltf_sigs in ltf_signals:
            bar = self.ltf_bars[idx]
            dt = _get_dt(bar)
            if dt is None:
                continue

            date_key = dt.strftime("%Y-%m-%d")
            daily_sigs = daily_context.get(date_key, [])

            # Merge and sort all component names
            all_components = sorted(set(ltf_sigs + daily_sigs))
            compounds.append(CompoundSignal(
                components=tuple(all_components),
                timestamp=dt,
                bar_close=float(_get(bar, "close")),
                bar_index=idx,
            ))
        return compounds

    # ---- forward returns -------------------------------------------- #

    def compute_forward_returns(
        self,
        bar_index: int,
        bar_close: float,
    ) -> Dict[str, Optional[float]]:
        """
        Compute the % price change from ``bar_close`` at each horizon.

        Returns None for horizons that extend past the available data.
        """
        result: Dict[str, Optional[float]] = {}
        for name, n_bars in self.horizons.items():
            target_idx = bar_index + n_bars
            if target_idx >= len(self.ltf_bars):
                result[name] = None
            else:
                future_close = float(_get(self.ltf_bars[target_idx], "close"))
                result[name] = (future_close - bar_close) / bar_close
        return result

    # ---- main build ------------------------------------------------- #

    def build(self) -> PDFDocument:
        """
        Full pipeline: detect signals → compute forward returns → build
        distributions → return PDFDocument.
        """
        daily_ctx = self._build_daily_context()
        ltf_sigs = self._detect_ltf_signals()
        compounds = self.build_compound_signals(ltf_sigs, daily_ctx)

        # Group forward returns by compound key
        returns_by_key: Dict[str, Dict[str, List[float]]] = defaultdict(
            lambda: defaultdict(list)
        )

        for cs in compounds:
            fwd = self.compute_forward_returns(cs.bar_index, cs.bar_close)
            for horizon, ret in fwd.items():
                if ret is not None:
                    returns_by_key[cs.key][horizon].append(ret)

        # Build SignalPDF for each key
        signal_pdfs: Dict[str, SignalPDF] = {}
        for key, horizon_returns in returns_by_key.items():
            # Use the first available horizon for CI width calculation
            all_returns: List[float] = []
            for rets in horizon_returns.values():
                all_returns.extend(rets)

            ci = ci_95_width(all_returns)
            sufficient = ci < self.min_ci_width

            horizons: Dict[str, HorizonStats] = {}
            for h_name, rets in horizon_returns.items():
                if not rets:
                    continue

                if self.return_model != "empirical":
                    from lib.return_models import create_return_model
                    model = create_return_model(self.return_model)
                    model.fit(rets)
                    model_result = model.result()
                    horizons[h_name] = HorizonStats(
                        mean=model_result.mean,
                        stddev=model_result.stddev,
                        percentiles=model_result.percentiles,
                        forward_returns=rets,
                        model_name=model_result.model_name,
                        model_params={},
                    )
                else:
                    mean = sum(rets) / len(rets)
                    variance = sum((r - mean) ** 2 for r in rets) / max(len(rets) - 1, 1)
                    std = math.sqrt(variance)
                    pcts = compute_percentiles(rets)
                    horizons[h_name] = HorizonStats(
                        mean=mean,
                        stddev=std,
                        percentiles=pcts,
                        forward_returns=rets,
                    )

            signal_pdfs[key] = SignalPDF(
                sample_size=len(all_returns),
                ci_95_width=ci,
                sufficient_samples=sufficient,
                horizons=horizons,
            )

        # Determine scan range from bar timestamps
        first_dt = _get_dt(self.ltf_bars[0]) if self.ltf_bars else None
        last_dt = _get_dt(self.ltf_bars[-1]) if self.ltf_bars else None

        return PDFDocument(
            symbol=self.symbol,
            signals=signal_pdfs,
            scan_start=first_dt.isoformat() if first_dt else "",
            scan_end=last_dt.isoformat() if last_dt else "",
            min_ci_width_threshold=self.min_ci_width,
            ltf_period_seconds=self.ltf_period_seconds,
            atomic_signal_catalog=list(ATOMIC_SIGNAL_CATALOG),
        )
