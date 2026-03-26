"""
Base Strategy Interface.

All trading strategies inherit from this class and implement the required methods.
The engine calls on_tick() in a loop, using get_next_tick_seconds() to
determine pacing and should_fetch_account() for account state fetching.

Lifecycle hooks (on_retrain, on_complete) are optional overrides.
Strategies can receive pre-trained data via constructor kwargs to skip
retraining (D-04, D-05).
"""

from abc import ABC, abstractmethod


class BaseStrategy(ABC):
    """Common interface for all trading strategies.

    All strategies inherit from this class and implement the required methods.
    The engine calls on_tick() in a loop, using get_next_tick_seconds() to
    determine pacing and should_fetch_account() for account state fetching.

    Lifecycle hooks (on_retrain, on_complete) are optional overrides.
    Strategies can receive pre-trained data via constructor kwargs to skip
    retraining (D-04, D-05).
    """

    def __init__(self, playground, symbol: str, logger=None, **kwargs):
        self.playground = playground
        self.symbol = symbol
        self.logger = logger

    @abstractmethod
    def on_tick(self, tick_deltas) -> None:
        """Process one tick's worth of data.
        Strategy handles its own signal generation and order placement internally."""
        pass

    @abstractmethod
    def get_next_tick_seconds(self) -> int:
        """Return the number of seconds for the next tick (D-03).
        Enables smart HTF/LTF optimization."""
        pass

    @abstractmethod
    def should_fetch_account(self) -> bool:
        """Whether the engine should fetch account state on this tick."""
        pass

    def on_retrain(self) -> None:
        """Optional lifecycle hook for periodic retraining (D-04).
        Default: no-op. Override in strategies that need retraining.
        Can be skipped entirely via enable_retraining=False in run_strategy (D-05)."""
        pass

    def on_complete(self) -> None:
        """Optional lifecycle hook called when simulation completes.
        Default: no-op. Override for end-of-sim cleanup."""
        pass

    def is_complete(self) -> bool:
        """Check if the strategy/simulation is done.
        Default: delegates to playground.is_backtest_complete().
        Override if strategy has its own completion logic."""
        return self.playground.is_backtest_complete()
