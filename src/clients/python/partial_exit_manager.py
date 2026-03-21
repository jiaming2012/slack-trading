"""
Adaptive partial exit logic for mean-reversion strategies.

Given a set of filled entry levels and a profit target (HTF signal price),
computes exit tiers for partial position exits as price recovers.

Standalone module — no server dependency.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import List, Set, Tuple


# ------------------------------------------------------------------ #
# Data structures
# ------------------------------------------------------------------ #

@dataclass
class ExitTier:
    """One partial exit point."""

    exit_price: float
    shares_to_sell: int
    source_level_index: int  # which entry level these shares came from
    tier_index: int          # 0, 1, 2 within this level's tiers


@dataclass
class ExitPlan:
    """Complete exit plan across all filled levels."""

    tiers: List[ExitTier]


# ------------------------------------------------------------------ #
# Public API
# ------------------------------------------------------------------ #

def compute_exit_plan(
    filled_levels: List[Tuple[float, int]],
    signal_price: float,
    num_tiers: int = 3,
    tier_spacing: str = "even",
) -> ExitPlan:
    """
    Compute partial exit tiers for filled entry levels.

    For each filled level, creates ``num_tiers`` exit points between
    the entry price and the signal price (profit target). Shares are
    divided roughly equally across tiers using largest-remainder.

    Parameters
    ----------
    filled_levels : list of (entry_price, shares_filled)
        Each entry level that has been filled, with its fill price and
        number of shares bought.
    signal_price : float
        HTF signal price — the ultimate profit target.
    num_tiers : int
        Number of partial exit tiers per level (default 3).
    tier_spacing : str
        How tiers are spaced between entry and signal price:
        - ``"even"`` (default): evenly spaced (25%, 50%, 100%)
        - ``"tight"``: clustered near signal price (70%, 85%, 100%)
          More profit per successful reversion.

    Returns
    -------
    ExitPlan
        Plan with all exit tiers across all levels.
    """
    if not filled_levels or num_tiers <= 0:
        return ExitPlan(tiers=[])

    all_tiers: List[ExitTier] = []

    for level_idx, (entry_price, shares) in enumerate(filled_levels):
        # Guard: can't exit if target is at or below entry
        if signal_price <= entry_price or shares <= 0:
            continue

        distance = signal_price - entry_price

        # Split shares across tiers (largest-remainder)
        tier_shares = _split_shares(shares, num_tiers)

        for tier_idx in range(num_tiers):
            if tier_shares[tier_idx] <= 0:
                continue

            # Last tier exits at signal_price exactly
            if tier_idx == num_tiers - 1:
                exit_price = signal_price
            else:
                fraction = _tier_fraction(tier_idx, num_tiers, tier_spacing)
                exit_price = entry_price + fraction * distance

            all_tiers.append(ExitTier(
                exit_price=exit_price,
                shares_to_sell=tier_shares[tier_idx],
                source_level_index=level_idx,
                tier_index=tier_idx,
            ))

    return ExitPlan(tiers=all_tiers)


def check_exits(
    current_price: float,
    exit_plan: ExitPlan,
    already_triggered: Set[tuple],
) -> List[ExitTier]:
    """
    Check which exit tiers should trigger at the current price.

    Parameters
    ----------
    current_price : float
        Current market price (typically candle high for long positions).
    exit_plan : ExitPlan
        The exit plan to evaluate.
    already_triggered : set of (source_level_index, tier_index) tuples
        Tiers that have already been triggered, identified by their
        ``(source_level_index, tier_index)`` identity.

    Returns
    -------
    list of ExitTier
        Tiers that should trigger now (exit_price <= current_price and
        not already triggered). Sorted by exit_price ascending.
    """
    triggered = []
    for tier in exit_plan.tiers:
        tier_id = (tier.source_level_index, tier.tier_index)
        if tier_id in already_triggered:
            continue
        if current_price >= tier.exit_price:
            triggered.append(tier)
    return sorted(triggered, key=lambda t: t.exit_price)


# ------------------------------------------------------------------ #
# Internal helpers
# ------------------------------------------------------------------ #

def _tier_fraction(tier_idx: int, num_tiers: int, spacing: str) -> float:
    """
    Compute the fraction of distance (entry→signal) for a given tier.

    Parameters
    ----------
    tier_idx : int
        0-based tier index (last tier is always 1.0, handled by caller).
    num_tiers : int
        Total number of tiers.
    spacing : str
        ``"even"`` or ``"tight"``.

    Returns
    -------
    float
        Fraction in (0, 1).
    """
    if spacing == "tight":
        # Cluster tiers near signal_price.
        # For 3 tiers: fractions are 0.70, 0.85, 1.00
        # For N tiers: linearly space between a high starting point and 1.0
        # Start at 1 - 0.3*(N-1)/N, step by 0.3/N
        start = 1.0 - 0.3 * (num_tiers - 1) / num_tiers
        step = 0.3 / num_tiers
        return start + tier_idx * step
    else:
        # Even spacing (original): 1/(n+1), 2/(n+1), ...
        return (tier_idx + 1) / (num_tiers + 1)


def _split_shares(total: int, n: int) -> List[int]:
    """
    Split ``total`` shares into ``n`` roughly equal parts.

    Uses largest-remainder method. If total < n, only the last
    tier(s) get shares.
    """
    if n <= 0:
        return []
    if total <= 0:
        return [0] * n

    base = total // n
    remainder = total % n

    # Give the remainder to the LAST tiers (so final tier gets most)
    parts = [base] * n
    for i in range(remainder):
        parts[n - 1 - i] += 1

    return parts
