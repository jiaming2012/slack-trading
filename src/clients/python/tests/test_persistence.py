"""Unit tests for engine.persistence -- backtest run persistence.

Tests use mock psycopg2 connections so no real database is required.
"""

import json
from unittest.mock import MagicMock, call

import pytest

from engine.persistence import save_backtest_run


# ------------------------------------------------------------------
# Helpers
# ------------------------------------------------------------------

def _make_mock_conn(stats_row=None, inserted_id=1):
    """Create a mock psycopg2 connection with a mock cursor.

    Parameters
    ----------
    stats_row : tuple or None
        What v_playground_stats SELECT returns. None means no stats.
    inserted_id : int
        The id returned by INSERT ... RETURNING id.
    """
    mock_cursor = MagicMock()

    # fetchone is called twice: once for v_playground_stats, once for RETURNING id
    mock_cursor.fetchone.side_effect = [stats_row, (inserted_id,)]
    mock_cursor.__enter__ = MagicMock(return_value=mock_cursor)
    mock_cursor.__exit__ = MagicMock(return_value=False)

    mock_conn = MagicMock()
    mock_conn.cursor.return_value = mock_cursor

    return mock_conn, mock_cursor


# ------------------------------------------------------------------
# Tests
# ------------------------------------------------------------------

def test_save_backtest_run_inserts_correct_row():
    """save_backtest_run should INSERT a row into backtest_runs with correct values."""
    stats_row = (15, 1234.56, 0.60, 1.8)  # total_trades, total_pnl, win_rate, profit_factor
    mock_conn, mock_cursor = _make_mock_conn(stats_row=stats_row, inserted_id=42)

    params = {"max_loss_pct": 0.02, "stop_percentile": 0.95}

    result = save_backtest_run(
        playground_id="aaaa-bbbb-cccc",
        client_id="test-client",
        strategy_name="mean_reversion",
        parameters=params,
        starting_balance=100000.0,
        final_balance=101234.56,
        start_date="2025-06-01",
        end_date="2026-02-28",
        conn=mock_conn,
    )

    assert result == 42

    # Check the INSERT call (second execute call)
    insert_call = mock_cursor.execute.call_args_list[1]
    insert_sql = insert_call[0][0]
    insert_values = insert_call[0][1]

    assert "INSERT INTO backtest_runs" in insert_sql
    assert insert_values == (
        "aaaa-bbbb-cccc",
        "test-client",
        "mean_reversion",
        json.dumps(params),
        100000.0,
        101234.56,
        1234.56,   # total_pnl from stats
        0.60,      # win_rate from stats
        1.8,       # profit_factor from stats
        15,        # total_trades from stats
        "2025-06-01",
        "2026-02-28",
    )

    mock_conn.commit.assert_called_once()


def test_save_backtest_run_queries_v_playground_stats():
    """save_backtest_run should query v_playground_stats BEFORE inserting."""
    stats_row = (10, 500.0, 0.5, 1.2)
    mock_conn, mock_cursor = _make_mock_conn(stats_row=stats_row)

    save_backtest_run(
        playground_id="test-pg-id",
        client_id="c1",
        strategy_name="test",
        parameters={},
        starting_balance=10000.0,
        final_balance=10500.0,
        conn=mock_conn,
    )

    # First execute call should be the SELECT from v_playground_stats
    first_call = mock_cursor.execute.call_args_list[0]
    first_sql = first_call[0][0]
    assert "v_playground_stats" in first_sql
    assert first_call[0][1] == ("test-pg-id",)

    # Second execute call should be the INSERT
    second_call = mock_cursor.execute.call_args_list[1]
    assert "INSERT INTO backtest_runs" in second_call[0][0]


def test_parameters_serialized_as_json():
    """Parameters dict should be serialized as a JSON string for JSONB storage."""
    stats_row = (5, 100.0, 0.4, 0.9)
    mock_conn, mock_cursor = _make_mock_conn(stats_row=stats_row)

    nested_params = {"max_loss_pct": 0.02, "nested": {"a": 1, "b": [2, 3]}}

    save_backtest_run(
        playground_id="pg-1",
        client_id="c1",
        strategy_name="test",
        parameters=nested_params,
        starting_balance=10000.0,
        final_balance=10100.0,
        conn=mock_conn,
    )

    insert_call = mock_cursor.execute.call_args_list[1]
    insert_values = insert_call[0][1]

    # The 4th value (index 3) is the parameters JSONB
    params_value = insert_values[3]
    assert isinstance(params_value, str)
    assert json.loads(params_value) == nested_params


def test_save_backtest_run_handles_no_stats():
    """When v_playground_stats returns no row, insert with defaults."""
    mock_conn, mock_cursor = _make_mock_conn(stats_row=None, inserted_id=7)

    result = save_backtest_run(
        playground_id="pg-empty",
        client_id="c1",
        strategy_name="test",
        parameters={},
        starting_balance=10000.0,
        final_balance=10000.0,
        conn=mock_conn,
    )

    assert result == 7

    insert_call = mock_cursor.execute.call_args_list[1]
    insert_values = insert_call[0][1]

    # When stats is None: total_trades=0, total_pnl=0.0, win_rate=None, profit_factor=None
    assert insert_values[6] == 0.0   # total_pnl
    assert insert_values[7] is None  # win_rate
    assert insert_values[8] is None  # profit_factor
    assert insert_values[9] == 0     # total_trades
