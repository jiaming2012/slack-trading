"""
Post-simulation evaluation report for the mean-reversion strategy.

Compares expected vs realized profit per order and per group,
computing calibration and accuracy metrics.

Standalone module — no server dependency beyond reading order records.
"""

from __future__ import annotations

import math
from dataclasses import dataclass
from typing import Dict, List, Optional, Tuple

import pandas as pd

from mean_reversion_strategy import TradeGroup


# ------------------------------------------------------------------ #
# Report dataclass
# ------------------------------------------------------------------ #

@dataclass
class ReportMetrics:
    """Aggregate evaluation metrics."""

    mae: float                  # mean absolute error (expected vs realized)
    directional_accuracy: float # fraction where sign(expected) == sign(realized)
    total_expected: float       # sum of expected profits across groups
    total_realized: float       # sum of realized profits across groups
    num_groups: int


# ------------------------------------------------------------------ #
# Order-level report
# ------------------------------------------------------------------ #

def build_order_rows(
    trade_groups: List[TradeGroup],
    order_records: Optional[List[dict]] = None,
) -> pd.DataFrame:
    """
    Build a DataFrame of order-level records from trade groups.

    Parameters
    ----------
    trade_groups : list of TradeGroup
        All trade groups from the strategy run.
    order_records : list of dict, optional
        Raw order records from the playground (with fill prices).
        If None, uses the information stored on the groups themselves.

    Returns
    -------
    pd.DataFrame
        One row per order with expected/realized profit columns.
    """
    rows = []

    for group in trade_groups:
        # Entry orders
        for level_idx, shares in group.filled_levels.items():
            if level_idx >= len(group.deviation_plan.levels):
                continue
            level = group.deviation_plan.levels[level_idx]

            expected_exit = (
                group.htf_signal_price * level.p_revert
                + group.stop_price * (1 - level.p_revert)
            )
            expected_profit = shares * (expected_exit - level.price)

            rows.append({
                "group_id": group.group_id,
                "htf_signal": group.htf_signal_key,
                "signal_price": group.htf_signal_price,
                "action": "entry",
                "side": "BUY",
                "quantity": shares,
                "price": level.price,
                "sigma_distance": level.sigma_distance,
                "p_revert": level.p_revert,
                "expected_profit": expected_profit,
                "model_name": group.model_name,
                "stop_price": group.stop_price,
                "level_index": level_idx,
                "was_truncated": group.deviation_plan.was_truncated,
                "timestamp": group.htf_signal_timestamp,
            })

        # Exit orders (from exit plan)
        if group.exit_plan:
            for tier in group.exit_plan.tiers:
                tier_id = (tier.source_level_index, tier.tier_index)
                if tier_id not in group.triggered_exits:
                    continue
                rows.append({
                    "group_id": group.group_id,
                    "htf_signal": group.htf_signal_key,
                    "signal_price": group.htf_signal_price,
                    "action": "partial_exit",
                    "side": "SELL",
                    "quantity": tier.shares_to_sell,
                    "price": tier.exit_price,
                    "sigma_distance": None,
                    "p_revert": None,
                    "expected_profit": None,
                    "model_name": group.model_name,
                    "stop_price": group.stop_price,
                    "level_index": tier.source_level_index,
                    "was_truncated": group.deviation_plan.was_truncated,
                    "timestamp": group.htf_signal_timestamp,
                })

        # Stop-out record
        if group.status == "stopped_out":
            total_remaining = sum(group.filled_levels.values())
            if group.exit_plan:
                for tier in group.exit_plan.tiers:
                    tier_id = (tier.source_level_index, tier.tier_index)
                    if tier_id in group.triggered_exits:
                        total_remaining -= tier.shares_to_sell
            if total_remaining > 0:
                rows.append({
                    "group_id": group.group_id,
                    "htf_signal": group.htf_signal_key,
                    "signal_price": group.htf_signal_price,
                    "action": "stop_out",
                    "side": "SELL",
                    "quantity": total_remaining,
                    "price": group.stop_price,
                    "sigma_distance": None,
                    "p_revert": None,
                    "expected_profit": None,
                    "model_name": group.model_name,
                    "stop_price": group.stop_price,
                    "level_index": None,
                    "was_truncated": group.deviation_plan.was_truncated,
                    "timestamp": group.htf_signal_timestamp,
                })

    if not rows:
        return pd.DataFrame(columns=[
            "group_id", "htf_signal", "signal_price", "action", "side",
            "quantity", "price", "sigma_distance", "p_revert",
            "expected_profit", "model_name", "stop_price", "level_index",
            "was_truncated", "timestamp",
        ])

    return pd.DataFrame(rows)


# ------------------------------------------------------------------ #
# Group-level summary
# ------------------------------------------------------------------ #

def build_group_summary(
    trade_groups: List[TradeGroup],
) -> pd.DataFrame:
    """
    Build a group-level summary DataFrame.

    For each group, computes total expected profit (sum over entry levels),
    estimated realized profit (based on exit/stop prices vs entry prices),
    and prediction error.

    Returns
    -------
    pd.DataFrame
        One row per trade group.
    """
    rows = []

    for group in trade_groups:
        if not group.filled_levels:
            continue

        # Total expected profit across entries
        total_expected = 0.0
        total_entry_cost = 0.0

        for level_idx, shares in group.filled_levels.items():
            if level_idx >= len(group.deviation_plan.levels):
                continue
            level = group.deviation_plan.levels[level_idx]
            expected_exit = (
                group.htf_signal_price * level.p_revert
                + group.stop_price * (1 - level.p_revert)
            )
            total_expected += shares * (expected_exit - level.price)
            total_entry_cost += shares * level.price

        # Estimate realized profit from exits and stops
        total_realized = 0.0

        if group.exit_plan:
            for tier in group.exit_plan.tiers:
                tier_id = (tier.source_level_index, tier.tier_index)
                if tier_id not in group.triggered_exits:
                    continue
                # Find the entry price for this tier's source level
                source_idx = tier.source_level_index
                if source_idx < len(group.deviation_plan.levels):
                    entry_price = group.deviation_plan.levels[source_idx].price
                    total_realized += tier.shares_to_sell * (tier.exit_price - entry_price)

        if group.status == "stopped_out":
            # Remaining shares sold at stop price
            remaining_shares = sum(group.filled_levels.values())
            if group.exit_plan:
                for tier in group.exit_plan.tiers:
                    tier_id = (tier.source_level_index, tier.tier_index)
                    if tier_id in group.triggered_exits:
                        remaining_shares -= tier.shares_to_sell
            if remaining_shares > 0:
                # Approximate: use weighted average entry price
                avg_entry = total_entry_cost / sum(group.filled_levels.values())
                total_realized += remaining_shares * (group.stop_price - avg_entry)

        prediction_error = total_realized - total_expected
        prediction_accuracy = (
            max(0.0, 1.0 - abs(prediction_error / total_expected))
            if abs(total_expected) > 1e-10
            else 0.0
        )

        rows.append({
            "group_id": group.group_id,
            "htf_signal": group.htf_signal_key,
            "status": group.status,
            "levels_planned": group.deviation_plan.original_level_count,
            "levels_filled": len(group.filled_levels),
            "total_expected_profit": total_expected,
            "total_realized_profit": total_realized,
            "prediction_error": prediction_error,
            "prediction_accuracy": prediction_accuracy,
        })

    if not rows:
        return pd.DataFrame(columns=[
            "group_id", "htf_signal", "status", "levels_planned",
            "levels_filled", "total_expected_profit", "total_realized_profit",
            "prediction_error", "prediction_accuracy",
        ])

    return pd.DataFrame(rows)


# ------------------------------------------------------------------ #
# Metrics
# ------------------------------------------------------------------ #

def compute_metrics(group_summary: pd.DataFrame) -> ReportMetrics:
    """
    Compute aggregate evaluation metrics from the group summary.

    Parameters
    ----------
    group_summary : pd.DataFrame
        Output of ``build_group_summary()``.

    Returns
    -------
    ReportMetrics
    """
    if group_summary.empty:
        return ReportMetrics(
            mae=0.0, directional_accuracy=0.0,
            total_expected=0.0, total_realized=0.0, num_groups=0,
        )

    expected = group_summary["total_expected_profit"]
    realized = group_summary["total_realized_profit"]

    mae = float((expected - realized).abs().mean())

    # Directional accuracy: sign(expected) == sign(realized)
    # Treat zero as matching any sign
    signs_match = (
        (expected >= 0) & (realized >= 0)
    ) | (
        (expected < 0) & (realized < 0)
    )
    directional_accuracy = float(signs_match.mean())

    return ReportMetrics(
        mae=mae,
        directional_accuracy=directional_accuracy,
        total_expected=float(expected.sum()),
        total_realized=float(realized.sum()),
        num_groups=len(group_summary),
    )


# ------------------------------------------------------------------ #
# Full report generator
# ------------------------------------------------------------------ #

def generate_mean_reversion_report(
    trade_groups: List[TradeGroup],
    output_path: Optional[str] = None,
) -> Tuple[pd.DataFrame, pd.DataFrame, ReportMetrics]:
    """
    Generate the full post-simulation evaluation report.

    Parameters
    ----------
    trade_groups : list of TradeGroup
        All trade groups from the strategy run.
    output_path : str, optional
        If provided, saves order-level CSV to this path.

    Returns
    -------
    (order_df, group_df, metrics)
    """
    order_df = build_order_rows(trade_groups)
    group_df = build_group_summary(trade_groups)
    metrics = compute_metrics(group_df)

    if output_path and not order_df.empty:
        order_df.to_csv(output_path, index=False)

    return order_df, group_df, metrics
