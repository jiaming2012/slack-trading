"""
Credit Spread Visualizations.

Two key plots for understanding credit spread behavior:

1. P&L Surface: Theoretical P&L heatmap (stock price × DTE) showing how
   a spread's value evolves over time and across stock prices.

2. Spread Value Decay Curves: Actual mark-to-market spread values over
   the holding period for each trade, collected during backtester simulation.

Usage:
    # After running the strategy:
    from credit_spread_visualizations import plot_pl_surface, SpreadValueTracker

    # 1. P&L surface for a specific entry
    plot_pl_surface(entry, group.direction, output_path="pl_surface.html")

    # 2. Decay curves (requires on_tick data collection)
    tracker = SpreadValueTracker()
    # ... pass tracker.on_tick as callback during strategy run ...
    tracker.plot(output_path="decay_curves.html")
"""

from __future__ import annotations

import numpy as np
from dataclasses import dataclass, field
from datetime import datetime
from typing import Dict, List, Optional, Tuple


@dataclass
class SpreadSnapshot:
    """A single point-in-time observation of a spread's value."""
    timestamp: datetime
    candles_held: int
    days_held: float
    dte: Optional[int]
    spread_value: Optional[float]  # cost to close (short_price - long_price)
    stock_price: float
    short_price: float
    long_price: float
    credit_per_contract: float


@dataclass
class SpreadTrack:
    """Full lifecycle of one spread entry."""
    group_id: str
    level_index: int
    direction: str
    short_strike: float
    long_strike: float
    spread_width: float
    net_credit: float
    credit_per_contract: float
    entry_stock_price: float
    entry_timestamp: datetime
    dte_at_entry: Optional[int]
    exit_reason: Optional[str] = None
    snapshots: List[SpreadSnapshot] = field(default_factory=list)


class SpreadValueTracker:
    """Collects spread value snapshots during simulation via on_tick callback.

    Usage:
        tracker = SpreadValueTracker()
        strategy = run_credit_spread_strategy(
            ...,
            on_tick=tracker.on_tick,
        )
        tracker.plot("decay_curves.html")
    """

    def __init__(self):
        self.tracks: Dict[str, SpreadTrack] = {}  # key: "{group_id}_{level_idx}"
        self._sample_interval = 6  # record every N ticks (every 30 min at 5-min candles)
        self._tick_count = 0

    def on_tick(self, strategy, tick_deltas) -> None:
        """Called on every tick by the strategy loop."""
        self._tick_count += 1
        if self._tick_count % self._sample_interval != 0:
            return

        playground = strategy.playground
        current_ts = playground.timestamp
        if current_ts is None:
            return

        stock_price = 0.0
        # Get stock price from positions or candles
        for sym, pos in playground.account.positions.items():
            if not sym.startswith("O:") and pos.current_price > 0:
                stock_price = pos.current_price
                break

        # If no stock position, try to get from the last LTF bar
        if stock_price == 0.0 and strategy._prev_ltf_bar:
            stock_price = strategy._prev_ltf_bar.get("close", 0.0)

        for group in strategy.trade_groups:
            if group.status not in ("pending", "active"):
                # Check if recently closed (might have final snapshot to record)
                pass

            for level_idx, entry in group.entries.items():
                key = f"{group.group_id}_{level_idx}"

                # Initialize track if new
                if key not in self.tracks:
                    self.tracks[key] = SpreadTrack(
                        group_id=group.group_id,
                        level_index=level_idx,
                        direction=group.direction,
                        short_strike=entry.short_leg.strike,
                        long_strike=entry.long_leg.strike,
                        spread_width=entry.spread_width,
                        net_credit=entry.net_credit,
                        credit_per_contract=(
                            entry.net_credit / (entry.contracts * 100)
                            if entry.contracts > 0 else 0
                        ),
                        entry_stock_price=entry.entry_stock_price,
                        entry_timestamp=entry.entry_timestamp,
                        dte_at_entry=entry.dte_at_entry,
                    )

                # Skip if already exited
                if level_idx in group.exited_entries:
                    continue

                # Get current option prices
                short_pos = playground.account.get_position(entry.short_leg.contract_symbol)
                long_pos = playground.account.get_position(entry.long_leg.contract_symbol)

                short_price = short_pos.current_price if short_pos else 0.0
                long_price = long_pos.current_price if long_pos else 0.0
                spread_value = (short_price - long_price) if (short_pos and long_pos) else None

                # Compute DTE
                dte = None
                if entry.short_leg.expiration_date:
                    try:
                        exp = datetime.fromisoformat(
                            entry.short_leg.expiration_date.replace("Z", "+00:00")
                        )
                        current_date = current_ts.date() if hasattr(current_ts, "date") else current_ts
                        dte = (exp.date() - current_date).days
                    except (ValueError, TypeError):
                        pass

                days_held = (current_ts - entry.entry_timestamp).total_seconds() / 86400
                credit_per = self.tracks[key].credit_per_contract

                self.tracks[key].snapshots.append(SpreadSnapshot(
                    timestamp=current_ts,
                    candles_held=entry.candles_held,
                    days_held=days_held,
                    dte=dte,
                    spread_value=spread_value,
                    stock_price=stock_price,
                    short_price=short_price,
                    long_price=long_price,
                    credit_per_contract=credit_per,
                ))

    def plot(self, output_path: str = "spread_decay_curves.html") -> str:
        """Generate interactive Plotly decay curves and save to HTML."""
        try:
            import plotly.graph_objects as go
            from plotly.subplots import make_subplots
        except ImportError:
            raise ImportError("plotly is required: pip install plotly")

        tracks_with_data = [t for t in self.tracks.values() if len(t.snapshots) >= 2]
        if not tracks_with_data:
            print("No spread tracks with sufficient data to plot.")
            return output_path

        fig = make_subplots(
            rows=2, cols=1,
            subplot_titles=(
                "Spread Value Over Time (normalized to credit)",
                "Spread Value vs DTE",
            ),
            vertical_spacing=0.12,
        )

        # Color by exit outcome
        colors = {
            "profit_target": "green",
            "early_profit": "limegreen",
            "reversion": "blue",
            "strike_breach": "red",
            "max_loss": "darkred",
            "gamma_risk": "orange",
            "time_decay": "purple",
            "pre_expiration": "gray",
            "end_of_sim": "gray",
            None: "lightgray",
        }

        for track in tracks_with_data:
            snapshots = track.snapshots
            valid = [s for s in snapshots if s.spread_value is not None]
            if not valid:
                continue

            days = [s.days_held for s in valid]
            # Normalize spread value: 1.0 = full credit, 0.0 = no value (max profit)
            # Values > 1.0 mean the spread costs more to close than the credit received (loss)
            normalized = [s.spread_value / track.credit_per_contract
                         if track.credit_per_contract > 0 else 0
                         for s in valid]

            color = colors.get(track.exit_reason, "lightgray")
            label = f"{track.group_id[:8]} L{track.level_index} ({track.direction})"

            # Plot 1: value vs days held
            fig.add_trace(
                go.Scatter(
                    x=days, y=normalized,
                    mode="lines",
                    name=label,
                    line=dict(color=color, width=1),
                    opacity=0.6,
                    hovertemplate=(
                        f"<b>{label}</b><br>"
                        "Days held: %{x:.1f}<br>"
                        "Value/Credit: %{y:.2f}<br>"
                        "<extra></extra>"
                    ),
                    showlegend=False,
                ),
                row=1, col=1,
            )

            # Plot 2: value vs DTE
            dte_valid = [(s.dte, s.spread_value / track.credit_per_contract)
                        for s in valid if s.dte is not None and track.credit_per_contract > 0]
            if dte_valid:
                dtes, vals = zip(*dte_valid)
                fig.add_trace(
                    go.Scatter(
                        x=list(dtes), y=list(vals),
                        mode="lines",
                        name=label,
                        line=dict(color=color, width=1),
                        opacity=0.6,
                        showlegend=False,
                    ),
                    row=2, col=1,
                )

        # Add reference lines
        for row in [1, 2]:
            fig.add_hline(y=1.0, line_dash="dash", line_color="black",
                         annotation_text="breakeven", row=row, col=1)
            fig.add_hline(y=0.0, line_dash="dash", line_color="green",
                         annotation_text="max profit", row=row, col=1)

        # Add legend entries for colors
        for reason, color in colors.items():
            if reason is None:
                continue
            fig.add_trace(
                go.Scatter(
                    x=[None], y=[None],
                    mode="markers",
                    marker=dict(size=10, color=color),
                    name=reason.replace("_", " ").title(),
                    showlegend=True,
                ),
                row=1, col=1,
            )

        fig.update_layout(
            title="Credit Spread Value Decay Curves",
            height=900,
            template="plotly_white",
        )
        fig.update_xaxes(title_text="Days Held", row=1, col=1)
        fig.update_xaxes(title_text="DTE (Days to Expiration)", autorange="reversed", row=2, col=1)
        fig.update_yaxes(title_text="Spread Value / Credit Received", row=1, col=1)
        fig.update_yaxes(title_text="Spread Value / Credit Received", row=2, col=1)

        fig.write_html(output_path)
        print(f"Decay curves saved to {output_path}")
        return output_path


def plot_pl_surface(
    short_strike: float,
    long_strike: float,
    net_credit_per_contract: float,
    direction: str,
    dte_at_entry: int = 30,
    risk_free_rate: float = 0.05,
    implied_vol: float = 0.30,
    output_path: str = "pl_surface.html",
) -> str:
    """Generate a P&L surface heatmap for a credit spread.

    X-axis: stock price range around the strikes
    Y-axis: days to expiration (DTE)
    Color: P&L per contract

    Uses Black-Scholes approximation for option values at different
    stock prices and DTEs to show how the spread value evolves.

    Parameters
    ----------
    short_strike : float
        Short leg strike price.
    long_strike : float
        Long leg strike price.
    net_credit_per_contract : float
        Net credit received per contract at entry.
    direction : str
        "bullish" (bull put) or "bearish" (bear call).
    dte_at_entry : int
        Days to expiration at entry.
    risk_free_rate : float
        Risk-free interest rate for BS model.
    implied_vol : float
        Implied volatility for BS model.
    output_path : str
        Output HTML file path.
    """
    try:
        import plotly.graph_objects as go
    except ImportError:
        raise ImportError("plotly is required: pip install plotly")

    spread_width = abs(short_strike - long_strike)
    mid_strike = (short_strike + long_strike) / 2

    # Stock price range: ±2× spread width around the midpoint
    price_range = max(spread_width * 4, 20)
    stock_prices = np.linspace(mid_strike - price_range, mid_strike + price_range, 100)

    # DTE range: from entry to 0
    dtes = np.arange(dte_at_entry, -1, -1)

    # Build P&L grid
    pl_grid = np.zeros((len(dtes), len(stock_prices)))

    for i, dte in enumerate(dtes):
        for j, S in enumerate(stock_prices):
            T = max(dte / 365.0, 1e-6)

            if direction == "bullish":
                # Bull put spread: short put at higher strike, long put at lower
                short_val = _bs_put(S, short_strike, T, risk_free_rate, implied_vol)
                long_val = _bs_put(S, long_strike, T, risk_free_rate, implied_vol)
            else:
                # Bear call spread: short call at lower strike, long call at higher
                short_val = _bs_call(S, short_strike, T, risk_free_rate, implied_vol)
                long_val = _bs_call(S, long_strike, T, risk_free_rate, implied_vol)

            # Spread value = cost to close = short_value - long_value
            spread_value = short_val - long_val
            # P&L = credit received - cost to close
            pl_grid[i, j] = (net_credit_per_contract - spread_value) * 100

    # Cap P&L display at ±max_loss for better color scale
    max_credit = net_credit_per_contract * 100
    max_loss = (spread_width - net_credit_per_contract) * 100

    fig = go.Figure(data=go.Heatmap(
        z=pl_grid,
        x=stock_prices,
        y=dtes,
        colorscale=[
            [0.0, "darkred"],
            [0.3, "red"],
            [0.45, "lightyellow"],
            [0.5, "white"],
            [0.55, "lightgreen"],
            [0.7, "green"],
            [1.0, "darkgreen"],
        ],
        zmid=0,
        zmin=-max_loss,
        zmax=max_credit,
        colorbar=dict(title="P&L ($)"),
        hovertemplate=(
            "Stock: $%{x:.2f}<br>"
            "DTE: %{y}<br>"
            "P&L: $%{z:.2f}<br>"
            "<extra></extra>"
        ),
    ))

    # Add strike lines
    fig.add_vline(x=short_strike, line_dash="dash", line_color="red",
                  annotation_text=f"Short: ${short_strike:.0f}")
    fig.add_vline(x=long_strike, line_dash="dash", line_color="blue",
                  annotation_text=f"Long: ${long_strike:.0f}")

    spread_type = "Bull Put" if direction == "bullish" else "Bear Call"
    fig.update_layout(
        title=(
            f"{spread_type} Credit Spread P&L Surface<br>"
            f"<sub>Short ${short_strike:.0f} / Long ${long_strike:.0f}"
            f" | Credit: ${net_credit_per_contract:.2f}/contract"
            f" | IV: {implied_vol:.0%}</sub>"
        ),
        xaxis_title="Stock Price ($)",
        yaxis_title="Days to Expiration",
        height=600,
        template="plotly_white",
    )

    fig.write_html(output_path)
    print(f"P&L surface saved to {output_path}")
    return output_path


def plot_pl_surface_from_entry(entry, direction: str, **kwargs) -> str:
    """Convenience wrapper: build P&L surface from a CreditSpreadEntry."""
    credit_per = entry.net_credit / (entry.contracts * 100) if entry.contracts > 0 else 0
    return plot_pl_surface(
        short_strike=entry.short_leg.strike,
        long_strike=entry.long_leg.strike,
        net_credit_per_contract=credit_per,
        direction=direction,
        dte_at_entry=entry.dte_at_entry or 30,
        **kwargs,
    )


def plot_all_entries(strategy, output_dir: str = ".") -> List[str]:
    """Generate P&L surfaces for all entries in the strategy."""
    import os
    paths = []
    for group in strategy.trade_groups:
        for level_idx, entry in group.entries.items():
            filename = f"pl_surface_{group.group_id[:8]}_L{level_idx}.html"
            path = os.path.join(output_dir, filename)
            plot_pl_surface_from_entry(entry, group.direction, output_path=path)
            paths.append(path)
    return paths


# --------------------------------------------------------------------------- #
# Black-Scholes helpers
# --------------------------------------------------------------------------- #

def _bs_d1(S: float, K: float, T: float, r: float, sigma: float) -> float:
    return (np.log(S / K) + (r + 0.5 * sigma**2) * T) / (sigma * np.sqrt(T))


def _bs_d2(S: float, K: float, T: float, r: float, sigma: float) -> float:
    return _bs_d1(S, K, T, r, sigma) - sigma * np.sqrt(T)


def _norm_cdf(x):
    """Standard normal CDF using error function."""
    from math import erf, sqrt
    return 0.5 * (1.0 + erf(x / sqrt(2.0)))


def _bs_call(S: float, K: float, T: float, r: float, sigma: float) -> float:
    if T <= 0:
        return max(S - K, 0.0)
    d1 = _bs_d1(S, K, T, r, sigma)
    d2 = _bs_d2(S, K, T, r, sigma)
    return S * _norm_cdf(d1) - K * np.exp(-r * T) * _norm_cdf(d2)


def _bs_put(S: float, K: float, T: float, r: float, sigma: float) -> float:
    if T <= 0:
        return max(K - S, 0.0)
    d1 = _bs_d1(S, K, T, r, sigma)
    d2 = _bs_d2(S, K, T, r, sigma)
    return K * np.exp(-r * T) * _norm_cdf(-d2) - S * _norm_cdf(-d1)
