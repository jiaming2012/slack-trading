#!/Users/jamal/miniconda3/envs/grodt/bin/python
"""Provision Metabase dashboards for trading analytics.

Usage:
    python infra/provision-metabase.py --password <pw>
    python infra/provision-metabase.py --api-key <key>
    python infra/provision-metabase.py --url http://192.168.8.164:3001 --user jac475@cornell.edu --password <pw>

Re-runnable: updates existing dashboards/cards, creates missing ones.
"""
import argparse
import json
import sys

import requests


# ---------------------------------------------------------------------------
# Authentication & session helpers
# ---------------------------------------------------------------------------

def authenticate(base_url, email, password):
    """Authenticate via Metabase session API and return session token."""
    resp = requests.post(
        f"{base_url}/api/session",
        json={"username": email, "password": password},
    )
    resp.raise_for_status()
    return resp.json()["id"]


def create_session(base_url, api_key=None, email=None, password=None):
    """Return a requests.Session with appropriate auth headers."""
    s = requests.Session()
    if api_key:
        s.headers.update({"X-API-KEY": api_key})
    else:
        token = authenticate(base_url, email, password)
        s.headers.update({"X-Metabase-Session": token})
    return s


# ---------------------------------------------------------------------------
# Metabase resource helpers
# ---------------------------------------------------------------------------

def get_database_id(session, base_url, db_name="playground"):
    """Look up the Metabase internal database ID by name."""
    resp = session.get(f"{base_url}/api/database")
    resp.raise_for_status()
    data = resp.json()
    dbs = data.get("data", data) if isinstance(data, dict) else data
    match = next((d for d in dbs if d["name"] == db_name), None)
    if not match:
        raise RuntimeError(f"Database '{db_name}' not found in Metabase")
    return match["id"]


def find_card_by_name(session, base_url, name):
    """Search for an existing saved question (card) by exact name."""
    resp = session.get(f"{base_url}/api/card")
    resp.raise_for_status()
    cards = resp.json()
    return next((c for c in cards if c["name"] == name), None)


def find_dashboard_by_name(session, base_url, name):
    """Search for an existing dashboard by exact name."""
    resp = session.get(f"{base_url}/api/dashboard")
    resp.raise_for_status()
    dashboards = resp.json()
    return next((d for d in dashboards if d["name"] == name), None)


def upsert_card(session, base_url, payload):
    """Create or update a saved question (card). Returns card ID."""
    name = payload["name"]
    existing = find_card_by_name(session, base_url, name)
    if existing:
        resp = session.put(f"{base_url}/api/card/{existing['id']}", json=payload)
        _check(resp, f"update card '{name}'")
        print(f"  Updated card: {name}")
        return existing["id"]
    resp = session.post(f"{base_url}/api/card", json=payload)
    _check(resp, f"create card '{name}'")
    card_id = resp.json()["id"]
    print(f"  Created card: {name} (id={card_id})")
    return card_id


def upsert_dashboard(session, base_url, payload):
    """Create or update a dashboard. Returns dashboard ID."""
    name = payload["name"]
    existing = find_dashboard_by_name(session, base_url, name)
    if existing:
        resp = session.put(f"{base_url}/api/dashboard/{existing['id']}", json=payload)
        _check(resp, f"update dashboard '{name}'")
        print(f"  Updated dashboard: {name}")
        return existing["id"]
    resp = session.post(f"{base_url}/api/dashboard", json=payload)
    _check(resp, f"create dashboard '{name}'")
    dash_id = resp.json()["id"]
    print(f"  Created dashboard: {name} (id={dash_id})")
    return dash_id


def set_dashboard_cards(session, base_url, dashboard_id, cards_payload):
    """Replace all cards on a dashboard.

    Metabase v0.59+ expects ordered_cards in the dashboard PUT body.
    """
    resp = session.put(
        f"{base_url}/api/dashboard/{dashboard_id}",
        json={"dashcards": cards_payload},
    )
    _check(resp, f"set cards on dashboard {dashboard_id}")


def _check(resp, action):
    """Check response status; print body and exit on failure."""
    if not resp.ok:
        print(f"ERROR {action}: {resp.status_code}", file=sys.stderr)
        print(resp.text, file=sys.stderr)
        sys.exit(1)


# ---------------------------------------------------------------------------
# Card payload builders
# ---------------------------------------------------------------------------

TEMPLATE_TAGS = {
    "playground_id": {
        "id": "pg_filter",
        "name": "playground_id",
        "display-name": "Playground",
        "type": "text",
    }
}

PLAYGROUND_PARAMETER = {
    "id": "playground_filter",
    "name": "Playground",
    "slug": "playground",
    "type": "string/=",
    "sectionId": "string",
}

STRATEGY_TEMPLATE_TAGS = {
    "strategy_name": {
        "id": "strategy_filter",
        "name": "strategy_name",
        "display-name": "Strategy",
        "type": "text",
    }
}

STRATEGY_PARAMETER = {
    "id": "strategy_filter",
    "name": "Strategy",
    "slug": "strategy",
    "type": "string/=",
    "sectionId": "string",
}


def _make_card(name, db_id, sql, display="table", viz_settings=None):
    """Build a native SQL card payload."""
    payload = {
        "name": name,
        "dataset_query": {
            "database": db_id,
            "type": "native",
            "native": {
                "query": sql,
                "template-tags": TEMPLATE_TAGS,
            },
        },
        "display": display,
        "visualization_settings": viz_settings or {},
    }
    return payload


def _param_mapping(card_id):
    """Standard playground_filter -> template-tag mapping for a card."""
    return [
        {
            "parameter_id": "playground_filter",
            "card_id": card_id,
            "target": ["variable", ["template-tag", "playground_id"]],
        }
    ]


def _make_strategy_card(name, db_id, sql, display="table", viz_settings=None):
    """Build a native SQL card with strategy_name template tag."""
    payload = {
        "name": name,
        "dataset_query": {
            "database": db_id,
            "type": "native",
            "native": {
                "query": sql,
                "template-tags": STRATEGY_TEMPLATE_TAGS,
            },
        },
        "display": display,
        "visualization_settings": viz_settings or {},
    }
    return payload


def _strategy_param_mapping(card_id):
    """Strategy filter -> template-tag mapping for comparison cards."""
    return [
        {
            "parameter_id": "strategy_filter",
            "card_id": card_id,
            "target": ["variable", ["template-tag", "strategy_name"]],
        }
    ]


_next_dashcard_id = 0


def _dash_card(card_id, row, col, size_x, size_y):
    """Build a dashboard card entry with parameter mapping.

    Each new dashcard needs a unique negative id (Metabase v0.59+ API).
    """
    global _next_dashcard_id
    _next_dashcard_id -= 1
    return {
        "id": _next_dashcard_id,
        "card_id": card_id,
        "row": row,
        "col": col,
        "size_x": size_x,
        "size_y": size_y,
        "parameter_mappings": _param_mapping(card_id),
    }


def _strategy_dash_card(card_id, row, col, size_x, size_y):
    """Build a dashboard card entry with strategy parameter mapping."""
    global _next_dashcard_id
    _next_dashcard_id -= 1
    return {
        "id": _next_dashcard_id,
        "card_id": card_id,
        "row": row,
        "col": col,
        "size_x": size_x,
        "size_y": size_y,
        "parameter_mappings": _strategy_param_mapping(card_id),
    }


# ---------------------------------------------------------------------------
# Dashboard 1: Trading Performance
# ---------------------------------------------------------------------------

def build_trading_performance_cards(session, base_url, db_id):
    """Create all cards for the Trading Performance dashboard."""
    cards = {}

    # Summary stats -- 4 scalar cards
    cards["total_pnl"] = upsert_card(session, base_url, _make_card(
        "Trading: Total P&L", db_id,
        "SELECT total_pnl FROM v_playground_stats WHERE playground_id = {{playground_id}}::uuid",
        display="scalar",
    ))

    cards["win_rate"] = upsert_card(session, base_url, _make_card(
        "Trading: Win Rate", db_id,
        "SELECT ROUND(win_rate * 100, 1) FROM v_playground_stats WHERE playground_id = {{playground_id}}::uuid",
        display="scalar",
        viz_settings={"suffix": "%"},
    ))

    cards["profit_factor"] = upsert_card(session, base_url, _make_card(
        "Trading: Profit Factor", db_id,
        "SELECT ROUND(profit_factor, 2) FROM v_playground_stats WHERE playground_id = {{playground_id}}::uuid",
        display="scalar",
    ))

    cards["total_trades"] = upsert_card(session, base_url, _make_card(
        "Trading: Total Trades", db_id,
        "SELECT total_trades FROM v_playground_stats WHERE playground_id = {{playground_id}}::uuid",
        display="scalar",
    ))

    # P&L Over Time -- line chart
    cards["pnl_over_time"] = upsert_card(session, base_url, _make_card(
        "Trading: P&L Over Time", db_id,
        """SELECT order_time, realized_pl,
       SUM(realized_pl) OVER (ORDER BY order_time) AS cumulative_pnl
FROM v_order_pnl
WHERE playground_id = {{playground_id}}::uuid
  AND side IN ('buy', 'buy_to_open', 'sell_short', 'sell_to_open')
ORDER BY order_time""",
        display="line",
        viz_settings={
            "graph.x_axis.column": "order_time",
            "graph.metrics": ["cumulative_pnl"],
        },
    ))

    # Equity Curve with Drawdown
    cards["equity_curve"] = upsert_card(session, base_url, _make_card(
        "Trading: Equity Curve with Drawdown", db_id,
        """SELECT e.timestamp, e.equity,
       MAX(e.equity) OVER (ORDER BY e.timestamp) AS peak_equity,
       e.equity - MAX(e.equity) OVER (ORDER BY e.timestamp) AS drawdown
FROM equity_plot_records e
WHERE e.playground_session_id = {{playground_id}}::uuid
ORDER BY e.timestamp""",
        display="line",
        viz_settings={
            "graph.x_axis.column": "timestamp",
            "graph.metrics": ["equity", "drawdown"],
            "series_settings": {
                "drawdown": {"display": "area"},
            },
        },
    ))

    # Gross Profit / Gross Loss -- bar chart
    cards["gross_pnl"] = upsert_card(session, base_url, _make_card(
        "Trading: Gross Profit / Gross Loss", db_id,
        "SELECT gross_profit, gross_loss FROM v_playground_stats WHERE playground_id = {{playground_id}}::uuid",
        display="bar",
    ))

    # Win/Loss Breakdown -- bar chart
    cards["win_loss"] = upsert_card(session, base_url, _make_card(
        "Trading: Win/Loss Breakdown", db_id,
        "SELECT winners, losers, breakeven FROM v_playground_stats WHERE playground_id = {{playground_id}}::uuid",
        display="bar",
    ))

    return cards


def assemble_trading_performance(session, base_url, cards):
    """Create the Trading Performance dashboard and lay out cards."""
    dash_id = upsert_dashboard(session, base_url, {
        "name": "Trading Performance",
        "parameters": [PLAYGROUND_PARAMETER],
    })

    layout = [
        _dash_card(cards["total_pnl"], row=0, col=0, size_x=4, size_y=3),
        _dash_card(cards["win_rate"], row=0, col=4, size_x=4, size_y=3),
        _dash_card(cards["profit_factor"], row=0, col=8, size_x=4, size_y=3),
        _dash_card(cards["total_trades"], row=0, col=12, size_x=4, size_y=3),
        _dash_card(cards["pnl_over_time"], row=3, col=0, size_x=18, size_y=6),
        _dash_card(cards["equity_curve"], row=9, col=0, size_x=18, size_y=6),
        _dash_card(cards["gross_pnl"], row=15, col=0, size_x=9, size_y=4),
        _dash_card(cards["win_loss"], row=15, col=9, size_x=9, size_y=4),
    ]

    set_dashboard_cards(session, base_url, dash_id, layout)
    return dash_id


# ---------------------------------------------------------------------------
# Dashboard 2: Slippage Analysis
# ---------------------------------------------------------------------------

def build_slippage_cards(session, base_url, db_id):
    """Create all cards for the Slippage Analysis dashboard."""
    cards = {}

    cards["summary"] = upsert_card(session, base_url, _make_card(
        "Slippage: Summary", db_id,
        """SELECT slippage_type,
       COUNT(*) AS fills,
       ROUND(AVG(slippage_points), 4) AS avg_slippage_pts,
       ROUND(SUM(slippage_dollars), 2) AS total_slippage_dollars
FROM v_all_slippage
WHERE playground_id = {{playground_id}}::uuid
GROUP BY slippage_type
ORDER BY slippage_type""",
        display="table",
    ))

    cards["detail"] = upsert_card(session, base_url, _make_card(
        "Slippage: Per-Trade Detail", db_id,
        """SELECT order_id, symbol, side, slippage_type,
       requested_price, fill_price, fill_qty,
       ROUND(slippage_points, 4) AS slippage_pts,
       ROUND(slippage_dollars, 2) AS slippage_dollars
FROM v_all_slippage
WHERE playground_id = {{playground_id}}::uuid
ORDER BY order_id""",
        display="table",
    ))

    cards["distribution"] = upsert_card(session, base_url, _make_card(
        "Slippage: Distribution by Symbol", db_id,
        """SELECT symbol, slippage_type,
       ROUND(AVG(slippage_points), 4) AS avg_slippage,
       ROUND(SUM(slippage_dollars), 2) AS total_slippage
FROM v_all_slippage
WHERE playground_id = {{playground_id}}::uuid
GROUP BY symbol, slippage_type
ORDER BY total_slippage DESC""",
        display="bar",
        viz_settings={
            "graph.x_axis.column": "symbol",
            "graph.metrics": ["total_slippage"],
            "graph.dimensions": ["symbol", "slippage_type"],
        },
    ))

    return cards


def assemble_slippage_analysis(session, base_url, cards):
    """Create the Slippage Analysis dashboard and lay out cards."""
    dash_id = upsert_dashboard(session, base_url, {
        "name": "Slippage Analysis",
        "parameters": [PLAYGROUND_PARAMETER],
    })

    layout = [
        _dash_card(cards["summary"], row=0, col=0, size_x=18, size_y=5),
        _dash_card(cards["detail"], row=5, col=0, size_x=18, size_y=8),
        _dash_card(cards["distribution"], row=13, col=0, size_x=18, size_y=5),
    ]

    set_dashboard_cards(session, base_url, dash_id, layout)
    return dash_id


# ---------------------------------------------------------------------------
# Dashboard 3: Portfolio Analytics
# ---------------------------------------------------------------------------

def build_portfolio_cards(session, base_url, db_id):
    """Create all cards for the Portfolio Analytics dashboard."""
    cards = {}

    cards["per_symbol"] = upsert_card(session, base_url, _make_card(
        "Portfolio: Per-Symbol P&L Breakdown", db_id,
        """SELECT symbol, class,
       COUNT(*) AS trades,
       ROUND(SUM(realized_pl), 2) AS total_pnl,
       COUNT(*) FILTER (WHERE realized_pl > 0) AS winners,
       COUNT(*) FILTER (WHERE realized_pl < 0) AS losers,
       ROUND(SUM(realized_pl) FILTER (WHERE realized_pl > 0), 2) AS gross_profit,
       ROUND(SUM(realized_pl) FILTER (WHERE realized_pl < 0), 2) AS gross_loss
FROM v_order_pnl
WHERE playground_id = {{playground_id}}::uuid
  AND side IN ('buy', 'buy_to_open', 'sell_short', 'sell_to_open')
GROUP BY symbol, class
ORDER BY total_pnl DESC""",
        display="table",
    ))

    cards["position_history"] = upsert_card(session, base_url, _make_card(
        "Portfolio: Position History", db_id,
        """SELECT tf.order_time, tf.symbol, tf.class, tf.side, tf.status,
       tf.requested_price, tf.fill_price, tf.fill_qty, op.realized_pl
FROM v_trade_fills tf
LEFT JOIN v_order_pnl op ON op.order_id = tf.order_id
WHERE tf.playground_id = {{playground_id}}::uuid
ORDER BY tf.order_time""",
        display="table",
    ))

    cards["pnl_by_class"] = upsert_card(session, base_url, _make_card(
        "Portfolio: P&L by Asset Class", db_id,
        """SELECT class, ROUND(SUM(realized_pl), 2) AS total_pnl, COUNT(*) AS trades
FROM v_order_pnl
WHERE playground_id = {{playground_id}}::uuid
  AND side IN ('buy', 'buy_to_open', 'sell_short', 'sell_to_open')
GROUP BY class""",
        display="bar",
        viz_settings={
            "graph.x_axis.column": "class",
            "graph.metrics": ["total_pnl"],
        },
    ))

    cards["active_symbols"] = upsert_card(session, base_url, _make_card(
        "Portfolio: Active Symbols", db_id,
        """SELECT symbol, class, COUNT(*) AS total_orders,
       MIN(order_time) AS first_trade, MAX(order_time) AS last_trade
FROM v_order_pnl
WHERE playground_id = {{playground_id}}::uuid
GROUP BY symbol, class
ORDER BY total_orders DESC""",
        display="table",
    ))

    return cards


def assemble_portfolio_analytics(session, base_url, cards):
    """Create the Portfolio Analytics dashboard and lay out cards."""
    dash_id = upsert_dashboard(session, base_url, {
        "name": "Portfolio Analytics",
        "parameters": [PLAYGROUND_PARAMETER],
    })

    layout = [
        _dash_card(cards["per_symbol"], row=0, col=0, size_x=18, size_y=6),
        _dash_card(cards["position_history"], row=6, col=0, size_x=18, size_y=8),
        _dash_card(cards["pnl_by_class"], row=14, col=0, size_x=9, size_y=5),
        _dash_card(cards["active_symbols"], row=14, col=9, size_x=9, size_y=5),
    ]

    set_dashboard_cards(session, base_url, dash_id, layout)
    return dash_id


# ---------------------------------------------------------------------------
# Dashboard 4: Strategy Comparison
# ---------------------------------------------------------------------------

def build_strategy_comparison_cards(session, base_url, db_id):
    """Create all cards for the Strategy Comparison dashboard."""
    cards = {}

    cards["runs_table"] = upsert_card(session, base_url, _make_strategy_card(
        "Comparison: Backtest Runs", db_id,
        """SELECT br.client_id, br.strategy_name, br.starting_balance,
       br.final_balance, br.total_pnl, br.win_rate,
       br.profit_factor, br.total_trades,
       br.start_date, br.end_date, br.created_at
FROM backtest_runs br
WHERE ({{strategy_name}} = '' OR br.strategy_name = {{strategy_name}})
ORDER BY br.created_at DESC""",
        display="table",
    ))

    cards["equity_curves"] = upsert_card(session, base_url, _make_strategy_card(
        "Comparison: Equity Curves", db_id,
        """SELECT e.timestamp, e.equity, br.client_id
FROM equity_plot_records e
JOIN backtest_runs br ON br.playground_id = e.playground_session_id
WHERE ({{strategy_name}} = '' OR br.strategy_name = {{strategy_name}})
ORDER BY br.client_id, e.timestamp""",
        display="line",
        viz_settings={
            "graph.x_axis.column": "timestamp",
            "graph.metrics": ["equity"],
            "graph.dimensions": ["client_id"],
        },
    ))

    cards["parameters"] = upsert_card(session, base_url, _make_strategy_card(
        "Comparison: Parameter Values", db_id,
        """SELECT br.client_id, br.strategy_name, br.total_pnl, br.win_rate,
       br.profit_factor, br.parameters
FROM backtest_runs br
WHERE ({{strategy_name}} = '' OR br.strategy_name = {{strategy_name}})
ORDER BY br.total_pnl DESC""",
        display="table",
    ))

    cards["best_return"] = upsert_card(session, base_url, _make_strategy_card(
        "Comparison: Best Return", db_id,
        """SELECT br.client_id || ': $' || ROUND(br.total_pnl, 2)
FROM backtest_runs br
WHERE ({{strategy_name}} = '' OR br.strategy_name = {{strategy_name}})
ORDER BY br.total_pnl DESC
LIMIT 1""",
        display="scalar",
    ))

    cards["worst_return"] = upsert_card(session, base_url, _make_strategy_card(
        "Comparison: Worst Return", db_id,
        """SELECT br.client_id || ': $' || ROUND(br.total_pnl, 2)
FROM backtest_runs br
WHERE ({{strategy_name}} = '' OR br.strategy_name = {{strategy_name}})
ORDER BY br.total_pnl ASC
LIMIT 1""",
        display="scalar",
    ))

    return cards


def assemble_strategy_comparison(session, base_url, cards):
    """Create the Strategy Comparison dashboard and lay out cards."""
    dash_id = upsert_dashboard(session, base_url, {
        "name": "Strategy Comparison",
        "parameters": [STRATEGY_PARAMETER],
    })

    layout = [
        _strategy_dash_card(cards["best_return"], row=0, col=0, size_x=9, size_y=3),
        _strategy_dash_card(cards["worst_return"], row=0, col=9, size_x=9, size_y=3),
        _strategy_dash_card(cards["runs_table"], row=3, col=0, size_x=18, size_y=8),
        _strategy_dash_card(cards["equity_curves"], row=11, col=0, size_x=18, size_y=8),
        _strategy_dash_card(cards["parameters"], row=19, col=0, size_x=18, size_y=6),
    ]

    set_dashboard_cards(session, base_url, dash_id, layout)
    return dash_id


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(
        description="Provision Metabase dashboards for trading analytics",
    )
    parser.add_argument(
        "--url", default="http://192.168.8.164:3001",
        help="Metabase base URL (default: http://192.168.8.164:3001)",
    )
    parser.add_argument(
        "--user", default="jac475@cornell.edu",
        help="Metabase admin email (default: jac475@cornell.edu)",
    )
    parser.add_argument(
        "--password",
        help="Metabase admin password (required if --api-key not provided)",
    )
    parser.add_argument(
        "--api-key",
        help="Metabase API key (preferred over session auth)",
    )
    args = parser.parse_args()

    if not args.api_key and not args.password:
        parser.error("Either --api-key or --password is required")

    base_url = args.url.rstrip("/")

    # Authenticate
    print(f"Connecting to Metabase at {base_url} ...")
    session = create_session(
        base_url,
        api_key=args.api_key,
        email=args.user,
        password=args.password,
    )

    # Look up database
    db_id = get_database_id(session, base_url)
    print(f"Found database 'playground' (id={db_id})")

    # Dashboard 1: Trading Performance
    print("\n--- Trading Performance ---")
    tp_cards = build_trading_performance_cards(session, base_url, db_id)
    tp_dash = assemble_trading_performance(session, base_url, tp_cards)
    print(f"Dashboard ready: Trading Performance (id={tp_dash})")

    # Dashboard 2: Slippage Analysis
    print("\n--- Slippage Analysis ---")
    sl_cards = build_slippage_cards(session, base_url, db_id)
    sl_dash = assemble_slippage_analysis(session, base_url, sl_cards)
    print(f"Dashboard ready: Slippage Analysis (id={sl_dash})")

    # Dashboard 3: Portfolio Analytics
    print("\n--- Portfolio Analytics ---")
    pa_cards = build_portfolio_cards(session, base_url, db_id)
    pa_dash = assemble_portfolio_analytics(session, base_url, pa_cards)
    print(f"Dashboard ready: Portfolio Analytics (id={pa_dash})")

    # Dashboard 4: Strategy Comparison
    print("\n--- Strategy Comparison ---")
    sc_cards = build_strategy_comparison_cards(session, base_url, db_id)
    sc_dash = assemble_strategy_comparison(session, base_url, sc_cards)
    print(f"Dashboard ready: Strategy Comparison (id={sc_dash})")

    # Summary
    total_cards = len(tp_cards) + len(sl_cards) + len(pa_cards) + len(sc_cards)
    print(f"\nDone: 4 dashboards, {total_cards} cards provisioned.")
    print(f"  Trading Performance:  {base_url}/dashboard/{tp_dash}")
    print(f"  Slippage Analysis:    {base_url}/dashboard/{sl_dash}")
    print(f"  Portfolio Analytics:   {base_url}/dashboard/{pa_dash}")
    print(f"  Strategy Comparison:   {base_url}/dashboard/{sc_dash}")


if __name__ == "__main__":
    main()
