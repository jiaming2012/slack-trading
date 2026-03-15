"""
Post-simulation evaluation report for the mean-reversion strategy.

Standalone module — fetches order records from the server using only
a playground ID and twirp host. Reconstructs the report entirely from
order attributes (tags) set at placement time.

Usage:
    python mean_reversion_report.py --playground-id <UUID> --twirp-host http://localhost:5051
"""

from __future__ import annotations

import argparse
from dataclasses import dataclass
from typing import Dict, List, Optional, Tuple

import pandas as pd

from rpc.playground_twirp import PlaygroundServiceClient
from rpc.playground_pb2 import GetAccountRequest
from twirp.context import Context


# ------------------------------------------------------------------ #
# Report dataclass
# ------------------------------------------------------------------ #

@dataclass
class ReportMetrics:
    """Aggregate evaluation metrics."""

    mae: float                  # mean absolute error (expected vs realized)
    directional_accuracy: float # fraction where sign(expected) == sign(realized)
    total_expected_pl: float    # sum of expected P&L across groups
    total_realized_pl: float    # sum of realized P&L across groups
    num_groups: int


# ------------------------------------------------------------------ #
# Server fetch
# ------------------------------------------------------------------ #

def fetch_orders(twirp_host: str, playground_id: str) -> list:
    """Fetch all filled orders from the playground server."""
    client = PlaygroundServiceClient(twirp_host, timeout=120)
    request = GetAccountRequest(
        playground_id=playground_id,
        fetch_orders=True,
        status=["filled"],
    )
    response = client.GetAccount(ctx=Context(), request=request)
    return list(response.orders)


def _get_attr(order, key: str, default: str = "") -> str:
    """Get an attribute from a protobuf order's attributes map."""
    return order.attributes.get(key, default)


def _get_attr_float(order, key: str, default=0.0):
    """Get a float attribute from a protobuf order's attributes map."""
    val = order.attributes.get(key, "")
    if not val:
        return default
    try:
        return float(val)
    except (ValueError, TypeError):
        return default


def _fill_price(order) -> float:
    """Get the fill price from an order: use first trade's price, else order.price."""
    if order.trades:
        return order.trades[0].price
    return order.price


# ------------------------------------------------------------------ #
# Order-level report
# ------------------------------------------------------------------ #

COLUMNS = [
    "order_id", "group_id", "htf_signal", "signal_price", "action",
    "symbol", "side", "quantity", "fill_price", "requested_price",
    "sigma_distance", "p_revert", "expected_pl", "realized_pl",
    "model_name", "stop_price", "level_index", "exit_tier",
    "status", "timestamp",
]


def build_order_rows(orders: list) -> pd.DataFrame:
    """
    Build a DataFrame of order-level records from server order objects.

    Filters to orders that have a mean-reversion ``action`` attribute.
    All metadata is reconstructed from order attributes.

    Columns:
    - ``expected_pl``: model prediction at entry time (entries only)
    - ``realized_pl``: server-computed P&L (exits/stops only)

    Parameters
    ----------
    orders : list
        Protobuf Order objects from GetAccount(fetch_orders=True).

    Returns
    -------
    pd.DataFrame
        One row per order.
    """
    rows = []

    for order in orders:
        action = _get_attr(order, "action")
        if not action:
            continue  # not a mean-reversion order

        group_id = _get_attr(order, "group_id")
        fill_px = _fill_price(order)

        # expected_pl: from entry order attribute (set at placement time)
        expected_pl = _get_attr_float(order, "expected_profit", None)

        # realized_pl: server-authoritative P&L (set on sell orders after fill)
        realized_pl = order.pl if order.HasField("pl") else None

        row = {
            "order_id": order.id,
            "group_id": group_id,
            "htf_signal": _get_attr(order, "htf_signal"),
            "signal_price": _get_attr_float(order, "signal_price"),
            "action": action,
            "symbol": order.symbol,
            "side": order.side,
            "quantity": order.quantity,
            "fill_price": fill_px,
            "requested_price": order.requested_price,
            "sigma_distance": _get_attr_float(order, "sigma_distance", None),
            "p_revert": _get_attr_float(order, "p_revert", None),
            "expected_pl": expected_pl,
            "realized_pl": realized_pl,
            "model_name": _get_attr(order, "model_name"),
            "stop_price": _get_attr_float(order, "stop_price"),
            "level_index": _get_attr(order, "level_index") or None,
            "exit_tier": _get_attr(order, "exit_tier") or None,
            "status": order.status,
            "timestamp": order.create_date,
        }

        rows.append(row)

    if not rows:
        return pd.DataFrame(columns=COLUMNS)

    return pd.DataFrame(rows, columns=COLUMNS)


# ------------------------------------------------------------------ #
# Group-level summary
# ------------------------------------------------------------------ #

SUMMARY_COLUMNS = [
    "group_id", "htf_signal", "status",
    "num_entries", "num_exits",
    "total_expected_pl", "total_realized_pl",
    "prediction_error", "prediction_accuracy",
]


def build_group_summary(order_df: pd.DataFrame) -> pd.DataFrame:
    """
    Build a group-level summary from the order DataFrame.

    - ``total_expected_pl``: sum of ``expected_pl`` from entry orders
      (model prediction at placement time).
    - ``total_realized_pl``: sum of ``realized_pl`` from exit/stop orders
      (server-authoritative P&L).

    Parameters
    ----------
    order_df : pd.DataFrame
        Output of ``build_order_rows()``.

    Returns
    -------
    pd.DataFrame
        One row per trade group.
    """
    if order_df.empty:
        return pd.DataFrame(columns=SUMMARY_COLUMNS)

    rows = []
    for group_id, group_orders in order_df.groupby("group_id"):
        entries = group_orders[group_orders["action"] == "entry"]
        exits = group_orders[group_orders["action"].isin(
            ["partial_exit", "stop_out", "end_of_sim_close"]
        )]

        if entries.empty:
            continue

        # Expected P&L: sum from entry order tags
        total_expected_pl = entries["expected_pl"].sum()
        if pd.isna(total_expected_pl):
            total_expected_pl = 0.0

        # Realized P&L: use server-authoritative pl from exit orders
        realized_values = exits["realized_pl"].dropna()
        if not realized_values.empty:
            total_realized_pl = realized_values.sum()
        else:
            total_realized_pl = 0.0

        prediction_error = total_realized_pl - total_expected_pl
        prediction_accuracy = (
            max(0.0, 1.0 - abs(prediction_error / total_expected_pl))
            if abs(total_expected_pl) > 1e-10
            else 0.0
        )

        # Determine group status from order types present
        if any(exits["action"] == "stop_out"):
            status = "stopped_out"
        elif any(exits["action"] == "end_of_sim_close"):
            status = "closed_eod"
        elif not exits.empty:
            status = "closed"
        else:
            status = "active"

        htf_signal = entries.iloc[0]["htf_signal"] if not entries.empty else ""

        rows.append({
            "group_id": group_id,
            "htf_signal": htf_signal,
            "status": status,
            "num_entries": len(entries),
            "num_exits": len(exits),
            "total_expected_pl": total_expected_pl,
            "total_realized_pl": total_realized_pl,
            "prediction_error": prediction_error,
            "prediction_accuracy": prediction_accuracy,
        })

    if not rows:
        return pd.DataFrame(columns=SUMMARY_COLUMNS)

    return pd.DataFrame(rows, columns=SUMMARY_COLUMNS)


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
            total_expected_pl=0.0, total_realized_pl=0.0, num_groups=0,
        )

    expected = group_summary["total_expected_pl"]
    realized = group_summary["total_realized_pl"]

    mae = float((expected - realized).abs().mean())

    # Directional accuracy: sign(expected) == sign(realized)
    signs_match = (
        (expected >= 0) & (realized >= 0)
    ) | (
        (expected < 0) & (realized < 0)
    )
    directional_accuracy = float(signs_match.mean())

    return ReportMetrics(
        mae=mae,
        directional_accuracy=directional_accuracy,
        total_expected_pl=float(expected.sum()),
        total_realized_pl=float(realized.sum()),
        num_groups=len(group_summary),
    )


# ------------------------------------------------------------------ #
# Full report generator
# ------------------------------------------------------------------ #

def generate_mean_reversion_report(
    playground_id: str,
    twirp_host: str = "http://localhost:5051",
    output_path: Optional[str] = None,
) -> Tuple[pd.DataFrame, pd.DataFrame, ReportMetrics]:
    """
    Generate the full post-simulation evaluation report.

    Fetches all filled orders from the server, filters to mean-reversion
    orders (those with an ``action`` attribute), and builds order-level
    and group-level summaries.

    Parameters
    ----------
    playground_id : str
        UUID of the playground to report on.
    twirp_host : str
        Twirp server URL (default http://localhost:5051).
    output_path : str, optional
        If provided, saves order-level CSV to this path.

    Returns
    -------
    (order_df, group_df, metrics)
    """
    orders = fetch_orders(twirp_host, playground_id)
    order_df = build_order_rows(orders)
    group_df = build_group_summary(order_df)
    metrics = compute_metrics(group_df)

    if output_path and not order_df.empty:
        order_df.to_csv(output_path, index=False)

    return order_df, group_df, metrics


# ------------------------------------------------------------------ #
# CLI
# ------------------------------------------------------------------ #

if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Generate mean-reversion strategy report from a completed playground."
    )
    parser.add_argument("--playground-id", type=str, required=True)
    parser.add_argument("--twirp-host", type=str, default="http://localhost:5051")
    parser.add_argument("--output", type=str, default=None, help="CSV output path")

    args = parser.parse_args()

    order_df, group_df, metrics = generate_mean_reversion_report(
        args.playground_id, args.twirp_host, args.output,
    )

    print(f"\n{'=' * 60}")
    print("MEAN-REVERSION REPORT")
    print(f"{'=' * 60}")
    print(f"  Orders:              {len(order_df)}")
    print(f"  Groups:              {metrics.num_groups}")
    print(f"  Total expected P&L:  ${metrics.total_expected_pl:,.2f}")
    print(f"  Total realized P&L:  ${metrics.total_realized_pl:,.2f}")
    print(f"  MAE:                 ${metrics.mae:,.2f}")
    print(f"  Directional accuracy:{metrics.directional_accuracy:6.1%}")
    print(f"{'=' * 60}")

    if not group_df.empty:
        print("\nGroup Summary:")
        print(group_df.to_string(index=False))

    if args.output:
        print(f"\nOrder-level CSV saved to: {args.output}")
