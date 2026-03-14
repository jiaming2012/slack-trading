"""
Data structures for the PDF-Guided Wheel Strategy.

Defines compound signals, forward-return distributions (HorizonStats),
per-signal PDFs (SignalPDF), and the top-level PDFDocument that serializes
to / from JSON.
"""

from __future__ import annotations

import json
import math
from dataclasses import dataclass, field, asdict
from datetime import datetime
from typing import Dict, List, Optional, Tuple


# ------------------------------------------------------------------ #
# Atomic & Compound Signals
# ------------------------------------------------------------------ #

ATOMIC_SIGNAL_CATALOG: List[str] = [
    # Candlestick patterns (detected from OHLC via candlestick_patterns.py)
    "bearish_pin_bar",
    "bullish_pin_bar",
    "engulfing_bearish",
    "engulfing_bullish",
    "hammer",
    "doji",
    # StochRSI state
    "stochrsi_above_80",
    "stochrsi_below_20",
    # StochRSI crossovers (from proto bool fields)
    "stochrsi_cross_above_20",
    "stochrsi_cross_below_80",
    # SuperTrend transitions (detect superD_50_3 change between bars)
    "supertrend_flip_up",
    "supertrend_flip_down",
    # Daily SuperTrend context (from daily bar superD_50_3)
    "daily_supertrend_up",
    "daily_supertrend_down",
    # SMA crossovers (close crosses SMA)
    "sma_50_cross_above",
    "sma_50_cross_below",
    "sma_100_cross_above",
    "sma_100_cross_below",
    "sma_200_cross_above",
    "sma_200_cross_below",
]

# Default forward-return horizons (number of 15-min bars)
DEFAULT_HORIZONS: Dict[str, int] = {
    "1h": 4,
    "4h": 16,
    "1d": 26,   # 6.5 trading hours
    "2d": 52,
}


@dataclass
class CompoundSignal:
    """A set of co-occurring atomic signals at a single bar."""

    components: Tuple[str, ...]   # sorted atomic signal names
    timestamp: datetime
    bar_close: float              # price at signal bar close
    bar_index: int = 0            # index into the LTF bar array

    @property
    def key(self) -> str:
        """Pipe-delimited lookup key, e.g. 'bearish_pin_bar|daily_supertrend_up'."""
        return "|".join(self.components)


# ------------------------------------------------------------------ #
# Distribution statistics per horizon
# ------------------------------------------------------------------ #

@dataclass
class HorizonStats:
    """Empirical distribution of forward returns at a single horizon."""

    mean: float
    stddev: float
    percentiles: Dict[str, float]   # {"5": -0.0098, "10": ..., "25": ..., "50": ...}
    forward_returns: List[float]    # raw observations

    def to_dict(self) -> dict:
        return asdict(self)

    @classmethod
    def from_dict(cls, d: dict) -> HorizonStats:
        return cls(
            mean=d["mean"],
            stddev=d["stddev"],
            percentiles=d["percentiles"],
            forward_returns=d["forward_returns"],
        )


@dataclass
class SignalPDF:
    """Distribution data for a single compound signal key."""

    sample_size: int
    ci_95_width: float
    sufficient_samples: bool
    horizons: Dict[str, HorizonStats]   # "1h", "4h", "1d", "2d"

    def to_dict(self) -> dict:
        return {
            "sample_size": self.sample_size,
            "ci_95_width": self.ci_95_width,
            "sufficient_samples": self.sufficient_samples,
            "horizons": {k: v.to_dict() for k, v in self.horizons.items()},
        }

    @classmethod
    def from_dict(cls, d: dict) -> SignalPDF:
        return cls(
            sample_size=d["sample_size"],
            ci_95_width=d["ci_95_width"],
            sufficient_samples=d["sufficient_samples"],
            horizons={k: HorizonStats.from_dict(v) for k, v in d["horizons"].items()},
        )


# ------------------------------------------------------------------ #
# Top-level PDF document
# ------------------------------------------------------------------ #

@dataclass
class PDFDocument:
    """
    Complete PDF output: one per stock, serializable to JSON.

    Contains a mapping of compound-signal keys to their forward-return
    distributions at multiple horizons.
    """

    symbol: str
    signals: Dict[str, SignalPDF]
    scan_start: str
    scan_end: str
    generated_at: str = ""
    min_ci_width_threshold: float = 0.005
    ltf_period_seconds: int = 900
    htf_period_seconds: int = 86400
    atomic_signal_catalog: List[str] = field(default_factory=lambda: list(ATOMIC_SIGNAL_CATALOG))

    # ---- persistence ------------------------------------------------ #

    def save(self, path: str) -> None:
        """Write the PDF to a JSON file."""
        obj = {
            "symbol": self.symbol,
            "scan_start": self.scan_start,
            "scan_end": self.scan_end,
            "generated_at": self.generated_at or datetime.utcnow().isoformat(),
            "min_ci_width_threshold": self.min_ci_width_threshold,
            "ltf_period_seconds": self.ltf_period_seconds,
            "htf_period_seconds": self.htf_period_seconds,
            "atomic_signal_catalog": self.atomic_signal_catalog,
            "signals": {k: v.to_dict() for k, v in self.signals.items()},
        }
        with open(path, "w") as f:
            json.dump(obj, f, indent=2)

    @classmethod
    def load(cls, path: str) -> PDFDocument:
        """Load a PDF from a JSON file."""
        with open(path) as f:
            obj = json.load(f)
        return cls(
            symbol=obj["symbol"],
            scan_start=obj["scan_start"],
            scan_end=obj["scan_end"],
            generated_at=obj.get("generated_at", ""),
            min_ci_width_threshold=obj.get("min_ci_width_threshold", 0.005),
            ltf_period_seconds=obj.get("ltf_period_seconds", 900),
            htf_period_seconds=obj.get("htf_period_seconds", 86400),
            atomic_signal_catalog=obj.get("atomic_signal_catalog", list(ATOMIC_SIGNAL_CATALOG)),
            signals={k: SignalPDF.from_dict(v) for k, v in obj.get("signals", {}).items()},
        )

    # ---- queries ----------------------------------------------------- #

    def get_signal(self, key: str) -> Optional[SignalPDF]:
        """Look up a signal PDF by its compound key."""
        return self.signals.get(key)

    def get_sufficient_signals(self) -> Dict[str, SignalPDF]:
        """Return only signals with sufficient sample size."""
        return {k: v for k, v in self.signals.items() if v.sufficient_samples}

    def find_best_signal(
        self,
        components: List[str],
        min_ci_width: float = 0.01,
        min_components: int = 1,
    ) -> Optional[Tuple[str, "SignalPDF"]]:
        """
        Find the best matching signal PDF for a set of detected atomic signals.

        Tries the full compound key first, then progressively smaller subsets
        (down to single signals), returning the match with the most components
        that has sufficient data.

        Selection criteria (in priority order):
          1. Largest number of matching components (more specific = better)
          2. Among ties, largest sample size

        Parameters
        ----------
        components : list[str]
            Sorted list of atomic signal names detected on the current bar.
        min_ci_width : float
            Maximum CI width to consider a signal usable (default 0.01).
        min_components : int
            Minimum subset size to try (default 1 = single-signal fallback).

        Returns
        -------
        (key, SignalPDF) or None
            Best matching signal, or None if nothing qualifies.
        """
        from itertools import combinations

        if not components:
            return None

        # Try from largest subset to smallest
        for size in range(len(components), min_components - 1, -1):
            best_key: Optional[str] = None
            best_pdf: Optional[SignalPDF] = None
            best_samples = -1

            for combo in combinations(components, size):
                key = "|".join(combo)
                pdf_entry = self.signals.get(key)
                if pdf_entry is None:
                    continue
                if pdf_entry.ci_95_width > min_ci_width:
                    continue
                if pdf_entry.sample_size > best_samples:
                    best_samples = pdf_entry.sample_size
                    best_key = key
                    best_pdf = pdf_entry

            if best_pdf is not None:
                return best_key, best_pdf

        return None


# ------------------------------------------------------------------ #
# Helpers
# ------------------------------------------------------------------ #

def ci_95_width(returns: List[float]) -> float:
    """
    Width of the 95 % confidence interval for the mean.

    CI width = 2 * 1.96 * std / sqrt(n).
    Returns inf when n < 2.
    """
    n = len(returns)
    if n < 2:
        return float("inf")
    mean = sum(returns) / n
    variance = sum((r - mean) ** 2 for r in returns) / (n - 1)
    std = math.sqrt(variance)
    return 2 * 1.96 * std / math.sqrt(n)


def compute_percentiles(
    returns: List[float],
    quantiles: Tuple[int, ...] = (5, 10, 25, 50),
) -> Dict[str, float]:
    """
    Compute percentiles using linear interpolation (numpy-free).

    Returns a dict keyed by string percentile, e.g. {"5": -0.009, ...}.
    """
    if not returns:
        return {str(q): 0.0 for q in quantiles}
    sorted_r = sorted(returns)
    n = len(sorted_r)
    result: Dict[str, float] = {}
    for q in quantiles:
        pos = (q / 100) * (n - 1)
        lo = int(pos)
        hi = min(lo + 1, n - 1)
        frac = pos - lo
        result[str(q)] = sorted_r[lo] * (1 - frac) + sorted_r[hi] * frac
    return result
