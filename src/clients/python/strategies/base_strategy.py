"""
Base Strategy Interface.

All trading strategies inherit from this class and implement the required methods.
The engine calls on_tick() in a loop, using get_next_tick_seconds() to
determine pacing and should_fetch_account() for account state fetching.

Lifecycle hooks (on_retrain, on_complete) are optional overrides.
Strategies can receive pre-trained data via constructor kwargs to skip
retraining (D-04, D-05).

Signal decision logging: strategies call self.record_decision() inside
on_tick() to record SignalDecision objects. The engine calls
_flush_decisions() after on_tick() to emit structured logs (Plan 03 wires this).
"""

import logging
import os
from abc import ABC, abstractmethod
from dataclasses import asdict

from opentelemetry import trace, metrics

from engine.types import SignalDecision


_signal_logger = logging.getLogger("grodt.strategy.signal")
_meter = metrics.get_meter("grodt-strategy")
_signals_counter = _meter.create_counter(
    "grodt.signals.generated",
    description="Number of signal decisions recorded by strategies",
)


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
        self._decisions: list = []

    @abstractmethod
    def on_tick(self, tick_deltas) -> None:
        """Process one tick's worth of data.
        Strategy handles its own signal generation and order placement internally.
        Call self.record_decision() to record signal decisions for logging."""
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

    def record_decision(self, decision: SignalDecision) -> None:
        """Record a signal decision for later logging.

        Auto-fills trace_id from the current OTel span context,
        playground_id from self.playground.id, and symbol from self.symbol
        if they are not already set.

        Strategies call this inside on_tick() to record decisions.
        """
        # Auto-fill trace_id from active OTel span
        if not decision.trace_id:
            span = trace.get_current_span()
            ctx = span.get_span_context()
            if ctx.trace_id > 0:
                decision.trace_id = format(ctx.trace_id, '032x')

        # Auto-fill playground_id
        if not decision.playground_id:
            decision.playground_id = str(self.playground.id)

        # Auto-fill symbol
        if not decision.symbol:
            decision.symbol = self.symbol

        self._decisions.append(decision)

    def _log_decision(self, decision: SignalDecision) -> None:
        """Log a single signal decision as a structured log record.

        When STRATEGY_LOG_VERBOSE=true, includes the full indicators dict.
        Otherwise, the indicators field is stripped from the log output (D-02).
        """
        record = asdict(decision)
        verbose = os.getenv("STRATEGY_LOG_VERBOSE", "false").lower() == "true"
        if not verbose:
            record.pop("indicators", None)

        _signal_logger.info("signal_decision", extra=record)

    def _flush_decisions(self) -> None:
        """Log all accumulated decisions and clear the list.

        Called by the engine AFTER on_tick() completes (Plan 03 wires this).
        This keeps logging DRY -- strategies only call record_decision(),
        and the engine handles emission.
        """
        for decision in self._decisions:
            self._log_decision(decision)
            _signals_counter.add(1, {
                "signal_type": decision.signal_type,
                "decision": decision.decision,
            })
        self._decisions = []
