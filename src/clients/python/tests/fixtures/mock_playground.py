"""Shared, contract-faithful fake playground for strategy tests.

Why this exists
---------------
The three strategy test suites (``mean_reversion``, ``options_mean_reversion``,
``credit_spread``) previously each built their playground double as a bare
``unittest.mock.MagicMock``. A bare MagicMock returns a *new* child mock for
every attribute read, so the moment a strategy constructor evaluated an
un-stubbed accessor in a typed expression -- e.g. ``current_price > 0`` where
``current_price`` came from an unset ``current_candles`` -- Python raised
``TypeError: '>' not supported between 'MagicMock' and 'int'`` and the test died
at *construction*, before any assertion ran. That was the root cause of 147 of
the 179 baseline failures.

This module replaces the bare MagicMock with a hand-written fake that:

* stubs every accessor the three strategy constructors and tick paths read,
  each with a correctly-typed default,
* lets every accessor be overridden per-test (plain attribute assignment),
* raises ``AttributeError`` on any *unknown* accessor rather than silently
  minting a child mock -- so when a strategy grows a new required playground
  accessor, the affected tests fail loudly at construction/first-use instead of
  quietly doing MagicMock arithmetic. That keeps the fake honest over time.

Accessor contract (empirically derived from construction + tick of the three
strategies in ``deprecated/{mean_reversion,options_mean_reversion,credit_spread}.py``;
cross-checked against the live client in ``engine/client.py``)
--------------------------------------------------------------------------------
Playground:
    ``htf_seconds: int``            HTF repository period, seconds (dispatch key)
    ``ltf_seconds: int``            LTF repository period, seconds (dispatch key)
    ``account``                     FakeAccount (see below)
    ``current_candles: dict``       {symbol: {period_seconds: bar}} -- auto-size reads
                                    ``current_candles.get(sym, {}).get(ltf_seconds).close``
    ``place_order``                 MagicMock -- order placement (tests assert calls)
    ``fetch_ladder``                MagicMock -- options ladder RPC (options/credit)
    ``flush_new_state_buffer``      MagicMock -> [] -- tick-loop drain
    ``tick``                        MagicMock -- advance clock
    ``get_realized_profit``         MagicMock -> 0.0
    ``is_backtest_complete: bool``  completion flag (mean_reversion reads directly)
    ``id: str``                     playground id (options/credit, record_decision)
    ``environment: str``            'simulator' | 'live'
    ``timestamp``                   datetime -- current sim time
    ``next_tick_at``                datetime -- live pacing
    ``profiler``                    None -- optional perf profiler (credit run loop)

Account (``playground.account``):
    ``balance: float``              cash balance -- auto-size divides by price
    ``equity: float``               account equity
    ``free_margin: float``          available margin
    ``positions: dict``             {symbol: FakePosition}
    ``meta``                        FakeAccountMeta (initial_balance, symbols)
    ``get_quantity(symbol) -> float``   MagicMock -> 0.0
    ``get_position(symbol) -> Position``MagicMock -> None
"""

from datetime import datetime
from unittest.mock import MagicMock
from zoneinfo import ZoneInfo


_NY = ZoneInfo("America/New_York")


class _StrictFake:
    """Base that raises AttributeError on reads of un-stubbed accessors.

    Attribute *assignment* is unaffected (normal ``__setattr__``), so tests can
    still override any stubbed accessor. Only *reads* of names that were never
    set raise -- preventing silent MagicMock leakage.
    """

    def __getattr__(self, name):  # only called when normal lookup fails
        raise AttributeError(
            f"{type(self).__name__} has no stubbed accessor '{name}'. "
            f"If production strategy code now reads it, add a typed default to "
            f"tests/fixtures/mock_playground.py rather than letting a MagicMock leak."
        )


class FakePosition(_StrictFake):
    """Typed position double for ``account.get_position`` overrides.

    Mirrors ``engine.client.Position`` fields so a test that needs a non-None
    position can build one with correct types instead of a bare MagicMock.
    """

    def __init__(
        self,
        symbol="AAPL",
        quantity=0.0,
        cost_basis=0.0,
        maintenance_margin=0.0,
        pl=0.0,
        current_price=0.0,
    ):
        self.symbol = symbol
        self.quantity = quantity
        self.cost_basis = cost_basis
        self.maintenance_margin = maintenance_margin
        self.pl = pl
        self.current_price = current_price


class FakeAccountMeta(_StrictFake):
    def __init__(self, initial_balance=100_000.0, symbols=None):
        self.initial_balance = initial_balance
        self.symbols = symbols if symbols is not None else []


class FakeAccount(_StrictFake):
    def __init__(self, equity=100_000.0, free_margin=None, balance=None, meta=None):
        self.equity = equity
        self.free_margin = free_margin if free_margin is not None else equity
        self.balance = balance if balance is not None else equity
        self.positions = {}
        self.meta = meta if meta is not None else FakeAccountMeta(initial_balance=self.balance)
        # Callable accessors as MagicMocks so tests can assert calls / override return_value.
        self.get_quantity = MagicMock(return_value=0.0)
        self.get_position = MagicMock(return_value=None)


class FakePlayground(_StrictFake):
    def __init__(
        self,
        equity=100_000.0,
        free_margin=None,
        balance=None,
        htf_seconds=3600,
        ltf_seconds=300,
        pg_id="test-pg-id",
        environment="simulator",
    ):
        self.htf_seconds = htf_seconds
        self.ltf_seconds = ltf_seconds
        self.id = pg_id
        self.environment = environment
        self.account = FakeAccount(equity=equity, free_margin=free_margin, balance=balance)
        # Real dict so auto-sizing's .get(...).get(...) yields None (not a mock),
        # driving the current_price==0 fallback path deterministically.
        self.current_candles = {}
        self.is_backtest_complete = False
        self.timestamp = datetime(2025, 6, 15, 10, 0, tzinfo=_NY)
        self.next_tick_at = datetime(2025, 6, 15, 10, 0, tzinfo=_NY)
        self.profiler = None
        # Callable accessors as MagicMocks (tests assert calls / set side_effect).
        self.place_order = MagicMock()
        self.fetch_ladder = MagicMock()
        self.flush_new_state_buffer = MagicMock(return_value=[])
        self.tick = MagicMock()
        self.get_realized_profit = MagicMock(return_value=0.0)


def make_mock_playground(equity=100_000.0, free_margin=None, balance=None, **overrides):
    """Build a contract-faithful fake playground.

    Mirrors the signatures of the per-file ``_make_mock_playground`` helpers the
    strategy test suites previously used (``equity`` / ``free_margin``), plus an
    optional ``balance`` and arbitrary keyword ``overrides`` applied as attribute
    assignments after construction (e.g. ``environment="live"``).
    """
    pg = FakePlayground(equity=equity, free_margin=free_margin, balance=balance)
    for key, value in overrides.items():
        setattr(pg, key, value)
    return pg
