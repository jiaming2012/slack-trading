"""
Position sizing via Kelly criterion with drawdown scaling.

Provides functions for:
- Computing the Kelly fraction from win probability and payoff ratio
- Adjusting for drawdown (fractional Kelly scaled by equity / peak)
- Allocating contracts across multiple strike prices
"""

from __future__ import annotations

import math
from typing import Dict, List, Tuple


def kelly_fraction(win_prob: float, win_loss_ratio: float) -> float:
    """
    Optimal Kelly fraction: f* = p - q / b.

    Parameters
    ----------
    win_prob : float
        Probability of a winning trade (0–1).
    win_loss_ratio : float
        Ratio of average win to average loss (b).

    Returns
    -------
    float
        Optimal fraction of capital to risk, clamped to [0, 1].
    """
    if win_loss_ratio <= 0 or win_prob <= 0:
        return 0.0
    q = 1.0 - win_prob
    f = win_prob - q / win_loss_ratio
    return max(0.0, min(f, 1.0))


def adjusted_kelly(
    f_star: float,
    current_equity: float,
    peak_equity: float,
    fraction: float = 0.5,
) -> float:
    """
    Fractional Kelly scaled by the drawdown ratio.

    adjusted = f* × fraction × (current_equity / peak_equity)

    This causes position sizes to shrink rapidly during drawdowns
    and grow slowly as equity recovers.

    Parameters
    ----------
    f_star : float
        Raw Kelly fraction (from :func:`kelly_fraction`).
    current_equity : float
        Current account equity.
    peak_equity : float
        Peak (high-water-mark) equity.
    fraction : float
        Kelly multiplier (default 0.5 = half-Kelly).

    Returns
    -------
    float
        Adjusted fraction of capital to risk, clamped to [0, 1].
    """
    if peak_equity <= 0 or current_equity <= 0:
        return 0.0
    dd_ratio = min(current_equity / peak_equity, 1.0)
    return max(0.0, min(f_star * fraction * dd_ratio, 1.0))


def max_contracts(
    adjusted_f: float,
    account_equity: float,
    margin_per_contract: float,
) -> int:
    """
    Maximum number of contracts for a given adjusted Kelly fraction.

    contracts = floor(adjusted_f × equity / margin_per_contract)

    Parameters
    ----------
    adjusted_f : float
        Adjusted Kelly fraction.
    account_equity : float
        Current account equity.
    margin_per_contract : float
        Capital required per contract (strike × 100 for cash-secured puts).

    Returns
    -------
    int
        Number of contracts (>= 0).
    """
    if margin_per_contract <= 0 or adjusted_f <= 0:
        return 0
    return int(adjusted_f * account_equity / margin_per_contract)


def allocate_contracts(
    total_contracts: int,
    probabilities: List[float],
) -> List[int]:
    """
    Distribute *total_contracts* across strike levels proportional to
    their probability of expiring OTM (i.e., more contracts at further-OTM
    / higher-probability strikes).

    Parameters
    ----------
    total_contracts : int
        Total contracts to allocate.
    probabilities : list[float]
        OTM probability for each strike (higher = further OTM).
        Must be in the same order as the strike list.

    Returns
    -------
    list[int]
        Number of contracts per strike. Sum equals *total_contracts*.
        Each strike gets at least 1 contract if total_contracts >= len(probabilities).
    """
    n = len(probabilities)
    if n == 0 or total_contracts <= 0:
        return [0] * n

    # Ensure each strike gets at least 1 if we have enough contracts
    if total_contracts >= n:
        allocation = [1] * n
        remaining = total_contracts - n
    else:
        # Not enough for one each — give to highest probability strikes
        allocation = [0] * n
        indices_by_prob = sorted(range(n), key=lambda i: probabilities[i], reverse=True)
        for i in range(total_contracts):
            allocation[indices_by_prob[i]] = 1
        return allocation

    # Distribute remaining proportionally to probability
    total_prob = sum(probabilities)
    if total_prob <= 0:
        # Equal distribution of remainder
        for i in range(remaining):
            allocation[i % n] += 1
        return allocation

    # Proportional allocation with largest-remainder method
    raw = [(p / total_prob) * remaining for p in probabilities]
    floors = [int(r) for r in raw]
    remainders = [r - f for r, f in zip(raw, floors)]

    for i in range(n):
        allocation[i] += floors[i]

    leftover = remaining - sum(floors)
    # Assign leftover to entries with largest fractional remainder
    indices_by_remainder = sorted(range(n), key=lambda i: remainders[i], reverse=True)
    for i in range(leftover):
        allocation[indices_by_remainder[i]] += 1

    return allocation
