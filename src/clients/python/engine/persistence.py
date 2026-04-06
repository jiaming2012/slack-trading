"""Backtest run persistence -- saves summary metrics to backtest_runs table.

Used by demo scripts when --save-to-db flag is passed. Requires psycopg2-binary.
"""

import json
import os
from typing import Optional

import psycopg2


def _get_connection():
    """Create a Postgres connection using environment variables."""
    return psycopg2.connect(
        host=os.getenv("POSTGRES_HOST", "localhost"),
        port=int(os.getenv("POSTGRES_PORT", "5432")),
        dbname=os.getenv("POSTGRES_DB", "playground"),
        user=os.getenv("POSTGRES_USER", "grodt"),
        password=os.getenv("POSTGRES_PASSWORD", "test747"),
    )


def save_backtest_run(
    playground_id: str,
    client_id: str,
    strategy_name: str,
    parameters: dict,
    starting_balance: float,
    final_balance: float,
    start_date: Optional[str] = None,
    end_date: Optional[str] = None,
    conn=None,
) -> int:
    """Save a backtest run summary to the backtest_runs table.

    Queries v_playground_stats for authoritative metrics (win_rate, profit_factor,
    total_trades, total_pnl) to stay consistent with existing dashboards.

    Parameters
    ----------
    playground_id : str
        UUID of the saved playground.
    client_id : str
        Human-readable identifier (e.g. "mean-reversion-aapl-2026-03-30").
    strategy_name : str
        Strategy label (e.g. "mean_reversion").
    parameters : dict
        Strategy constructor args as a dict (stored as JSONB).
    starting_balance : float
        Account balance before the backtest.
    final_balance : float
        Account balance after the backtest.
    start_date : str, optional
        Simulation start date (YYYY-MM-DD).
    end_date : str, optional
        Simulation end date (YYYY-MM-DD).
    conn : psycopg2 connection, optional
        Existing connection (for testing). Created automatically if None.

    Returns
    -------
    int
        The id of the inserted backtest_runs row.
    """
    own_conn = conn is None
    if own_conn:
        conn = _get_connection()

    try:
        with conn.cursor() as cur:
            # Verify analytics views exist before querying
            cur.execute(
                """SELECT 1 FROM information_schema.views
                   WHERE table_schema = 'public' AND table_name = 'v_playground_stats'"""
            )
            if cur.fetchone() is None:
                raise RuntimeError(
                    "View 'v_playground_stats' does not exist. "
                    "Run: task metabase:schema  (or apply infra/analytics-schema.sql manually)"
                )

            # Query v_playground_stats for authoritative metrics
            cur.execute(
                """SELECT total_trades, total_pnl, win_rate, profit_factor
                   FROM v_playground_stats
                   WHERE playground_id = %s""",
                (playground_id,),
            )
            stats = cur.fetchone()

            if stats:
                total_trades, total_pnl, win_rate, profit_factor = stats
            else:
                total_trades, total_pnl, win_rate, profit_factor = 0, 0.0, None, None

            cur.execute(
                """INSERT INTO backtest_runs
                   (playground_id, client_id, strategy_name, parameters,
                    starting_balance, final_balance, total_pnl,
                    win_rate, profit_factor, total_trades,
                    start_date, end_date)
                   VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                   RETURNING id""",
                (
                    playground_id,
                    client_id,
                    strategy_name,
                    json.dumps(parameters),
                    starting_balance,
                    final_balance,
                    total_pnl,
                    win_rate,
                    profit_factor,
                    total_trades,
                    start_date,
                    end_date,
                ),
            )
            row_id = cur.fetchone()[0]
            conn.commit()
            return row_id
    finally:
        if own_conn:
            conn.close()
