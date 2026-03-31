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
_flush_decisions() after on_tick() to emit structured logs and send
RecordSignal RPC to the server for telemetry.
"""

import logging
import os
from abc import ABC, abstractmethod
from dataclasses import asdict

from opentelemetry import trace

from engine.types import SignalDecision


_signal_logger = logging.getLogger("grodt.strategy.signal")


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
        self._rpc_client = None  # Set externally for RecordSignal RPC

    @abstractmethod
    def on_tick(self, tick_deltas) -> None:
        """Process one tick's worth of data.
        Strategy handles its own signal generation and order placement internally.
        Call self.record_decision() to record signal decisions for logging."""
        pass

    def on_signal(self, signal) -> None:
        """Handle a TradeSignal from TickDelta.new_signals.

        Override in V2 strategies. Default: no-op for backward compatibility.
        """
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

    def get_parameters(self) -> dict:
        """Return strategy tuning parameters as a dict for persistence.

        Override in subclasses to include strategy-specific parameters.
        Default returns empty dict so existing strategies are not broken.
        """
        return {}

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

    def _send_signal_rpc(self, decision: SignalDecision) -> None:
        """Send RecordSignal RPC to the server for telemetry.

        The server increments grodt_signals_generated_total in Prometheus.
        Falls back silently if no RPC client is configured.
        """
        client = self._rpc_client
        if client is None:
            # Try to get client from playground if it has one
            client = getattr(self.playground, '_client', None) or getattr(self.playground, 'client', None)

        if client is None:
            return

        try:
            from twirp.context import Context
            from rpc.playground_pb2 import RecordSignalRequest

            client.RecordSignal(ctx=Context(), request=RecordSignalRequest(
                playground_id=decision.playground_id,
                signal_type=decision.signal_type,
                direction=decision.direction,
                decision=decision.decision,
                reason=decision.reason,
                symbol=decision.symbol,
            ))
        except Exception as e:
            _signal_logger.debug("RecordSignal RPC failed (non-fatal): %s", e)

    def _flush_decisions(self) -> None:
        """Log all accumulated decisions, send to server, and clear the list.

        Called by the engine AFTER on_tick() completes (Plan 03 wires this).
        This keeps logging DRY -- strategies only call record_decision(),
        and the engine handles emission.
        """
        for decision in self._decisions:
            self._log_decision(decision)
            self._send_signal_rpc(decision)
        self._decisions = []
