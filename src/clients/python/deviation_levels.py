"""
PDF-adaptive deviation level placement and position sizing.

Given a forward-return distribution (from a PDFDocument), computes discrete
price levels below the signal price where the strategy should buy shares,
and determines how many shares at each level based on P(revert).

Standalone module — no server dependency.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import List, Optional

import numpy as np

from risk_management import allocate_contracts


# ------------------------------------------------------------------ #
# Data structures
# ------------------------------------------------------------------ #

@dataclass
class DeviationLevel:
    """One entry level in a deviation plan."""

    price: float           # entry price for this level
    shares: int            # shares to buy at this level
    sigma_distance: float  # how many σ below signal price
    p_revert: float        # probability of reverting to signal price


@dataclass
class DeviationPlan:
    """Complete plan for scaling into a position via deviation bands."""

    levels: List[DeviationLevel]
    signal_price: float
    stop_price: float
    max_potential_loss: float
    was_truncated: bool
    original_level_count: int


# ------------------------------------------------------------------ #
# Constants
# ------------------------------------------------------------------ #

_MIN_STDDEV = 1e-8
_MIN_SAMPLES = 3
_TAIL_THRESHOLD = 0.02  # stop generating levels when < 2% of returns reached


# ------------------------------------------------------------------ #
# Public API
# ------------------------------------------------------------------ #

def compute_deviation_levels(
    signal_price: float,
    stddev: float,
    forward_returns: List[float],
    stop_percentile: float,
    max_loss_budget: float,
    total_shares: int = 1000,
    sigma_steps: Optional[List[float]] = None,
    tail_threshold: Optional[float] = None,
) -> DeviationPlan:
    """
    Compute deviation levels for a mean-reversion entry plan.

    Parameters
    ----------
    signal_price : float
        Price at the HTF signal bar close (profit target / anchor).
    stddev : float
        Standard deviation of forward returns from the LTF PDF.
    forward_returns : list[float]
        Raw forward return observations from the PDF (as decimals,
        e.g. -0.02 = 2% decline).
    stop_percentile : float
        Percentile for stop-loss (e.g. 0.95 = 95th percentile adverse move).
    max_loss_budget : float
        Maximum allowable loss in dollars (X% of equity).
    total_shares : int
        Total shares to distribute across all levels.
    sigma_steps : list[float] or None
        Explicit sigma distances for levels. If None, auto-generated
        from the distribution tail.
    tail_threshold : float or None
        Override for _TAIL_THRESHOLD when auto-generating sigma steps.
        Lower values (e.g. 0.005) produce more/deeper levels.

    Returns
    -------
    DeviationPlan
        Plan with levels, stop price, and budget info. Returns an empty
        plan (no levels) if inputs are invalid.
    """
    empty = DeviationPlan(
        levels=[],
        signal_price=signal_price,
        stop_price=0.0,
        max_potential_loss=0.0,
        was_truncated=False,
        original_level_count=0,
    )

    # --- Input guards ---
    if stddev < _MIN_STDDEV:
        return empty
    if len(forward_returns) < _MIN_SAMPLES:
        return empty
    if total_shares <= 0 or max_loss_budget <= 0:
        return empty

    returns_arr = np.array(forward_returns, dtype=float)

    # --- Compute stop price (adverse tail) ---
    # For long positions, adverse = negative returns.
    # The (1 - stop_percentile)*100 percentile of returns gives the worst move.
    adverse_quantile = (1.0 - stop_percentile) * 100.0
    stop_return = float(np.percentile(returns_arr, adverse_quantile))
    stop_price = signal_price * (1.0 + stop_return)

    # --- Generate sigma steps if not provided ---
    if sigma_steps is None:
        thresh = tail_threshold if tail_threshold is not None else _TAIL_THRESHOLD
        sigma_steps = _auto_sigma_steps(returns_arr, stddev, tail_threshold=thresh)
    if not sigma_steps:
        return empty

    # --- Build candidate levels ---
    candidates = []
    for sigma in sorted(sigma_steps):
        level_price = signal_price * (1.0 - sigma * stddev)

        # Guard: skip levels where stop is at or above entry
        if stop_price >= level_price:
            continue

        p_revert = _compute_p_revert(returns_arr, sigma * stddev)
        candidates.append((level_price, sigma, p_revert))

    # Guard: all p_revert == 0 or no valid candidates
    if not candidates or all(c[2] == 0.0 for c in candidates):
        return empty

    original_count = len(candidates)

    # --- Allocate shares and enforce budget ---
    levels, was_truncated = _allocate_and_truncate(
        candidates, total_shares, stop_price, max_loss_budget,
    )

    if not levels:
        return empty

    max_loss = sum(lv.shares * (lv.price - stop_price) for lv in levels)

    return DeviationPlan(
        levels=levels,
        signal_price=signal_price,
        stop_price=stop_price,
        max_potential_loss=abs(max_loss),
        was_truncated=was_truncated,
        original_level_count=original_count,
    )


# ------------------------------------------------------------------ #
# Internal helpers
# ------------------------------------------------------------------ #

def _auto_sigma_steps(
    returns: np.ndarray,
    stddev: float,
    start: float = 1.0,
    step: float = 0.5,
    max_sigma: float = 5.0,
    tail_threshold: float = _TAIL_THRESHOLD,
) -> List[float]:
    """
    Auto-generate sigma steps from the distribution tail.

    Starts at ``start`` σ, increments by ``step``, stops when fewer
    than ``tail_threshold`` fraction of returns reached that level.
    """
    steps = []
    sigma = start
    n = len(returns)
    while sigma <= max_sigma:
        # What fraction of returns dipped at least this far?
        threshold = -sigma * stddev
        frac_reached = float(np.mean(returns <= threshold))
        if frac_reached < tail_threshold:
            break
        steps.append(sigma)
        sigma += step
    return steps


def _compute_p_revert(returns: np.ndarray, dip_magnitude: float) -> float:
    """
    Fraction of returns that dipped at least ``dip_magnitude`` but then
    recovered back to 0 (the signal price).

    Since we only have single-horizon returns (not intrabar paths), we
    approximate: a return counts as "reverted" if it ended >= 0 despite
    the dip level existing in the distribution.

    More precisely: P(revert | dipped to level) ≈
        P(return >= 0 AND return was at some point <= -dip_magnitude)

    With single-horizon data, we use a simpler proxy:
        P(revert from level) = P(return >= 0) among returns that are
        worse than -dip_magnitude OR that ended positive.

    Simplest sound approximation: among all returns, what fraction ended
    at or above 0?  Then weight by how deep we are.
    """
    # Returns that dipped to at least this level
    dipped = returns[returns <= -dip_magnitude]
    if len(dipped) == 0:
        # If no returns dipped this far, use overall positive fraction
        return float(np.mean(returns >= 0))

    # Of those that dipped this far, what fraction ultimately recovered?
    # With single-horizon data, "recovered" means the final return >= 0.
    # But dipped returns are by definition <= -dip_magnitude (negative),
    # so none of them recovered within the same horizon.
    #
    # Better proxy: use the full distribution's P(return >= 0) as a base,
    # scaled by the conditional probability of reaching this level.
    # P(revert) ≈ P(return >= 0) × (1 - P(return <= -dip)) / P(return <= -dip)
    #
    # Actually the cleanest approach for mean-reversion:
    # P(revert from level L) = fraction of ALL returns that ended >= 0.
    # Deeper levels get more shares because the _allocate step weights
    # by p_revert, and we adjust p_revert to be higher at deeper levels
    # where mean-reversion has more room to work.
    #
    # Use: P(revert) = P(return >= -dip_magnitude) — i.e., what fraction
    # of returns were NOT as bad as this level. This naturally increases
    # for deeper levels (more returns are "not that bad").
    p_not_as_bad = float(np.mean(returns >= -dip_magnitude))
    return p_not_as_bad


def _allocate_and_truncate(
    candidates: List[tuple],
    total_shares: int,
    stop_price: float,
    max_loss_budget: float,
) -> tuple:
    """
    Allocate shares across candidates, truncating deepest levels if
    max potential loss exceeds budget.

    Returns (levels, was_truncated).
    """
    was_truncated = False
    working = list(candidates)  # [(price, sigma, p_revert), ...]

    while working:
        probabilities = [c[2] for c in working]

        # If all p_revert are 0, can't allocate
        if all(p == 0.0 for p in probabilities):
            return [], False

        shares = allocate_contracts(total_shares, probabilities)

        levels = [
            DeviationLevel(
                price=c[0],
                shares=s,
                sigma_distance=c[1],
                p_revert=c[2],
            )
            for c, s in zip(working, shares)
        ]

        # Compute max potential loss
        max_loss = sum(
            lv.shares * abs(lv.price - stop_price) for lv in levels
        )

        if max_loss <= max_loss_budget:
            return levels, was_truncated

        # Over budget — remove deepest level (highest sigma)
        if len(working) <= 1:
            # Can't remove more; keep the single level
            return levels, True

        working.pop()  # remove deepest (sorted by sigma ascending)
        was_truncated = True

    return [], was_truncated
