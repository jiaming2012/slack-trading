"""
MA Crossover Datasource — produces trade signals from HTF bar data.

Extracts the signal detection logic from MeanReversionStrategy._process_htf_candle()
(lines 241-257) into a stateless, reusable function. This is the first datasource
module in the TradeSignal framework migration.

The caller is responsible for maintaining prev_bar state between calls.

Usage (sim mode — direct import)::

    from datasources.ma_crossover import produce_signals

    signals = produce_signals(bar_dict, prev_bar, pdf)
    for sig in signals:
        print(sig["signal_key"], sig["pdf_entry"])

Note: Live/RPC mode (WriteSignal endpoint) is deferred to Phase 19.
"""

from __future__ import annotations

from lib.pdf_builder import detect_atomic_signals_on_bar


def produce_signals(bar_dict: dict, prev_bar: dict | None, pdf) -> list:
    """Produce MA crossover trade signals from an HTF bar.

    Extracts the signal detection logic from MeanReversionStrategy._process_htf_candle()
    lines 241-257. Returns a list of signal dicts (empty if no valid signal).

    Args:
        bar_dict: Current HTF bar as dict (from _bar_to_dict).
        prev_bar: Previous HTF bar (needed by detect_atomic_signals_on_bar for
                  crossover detection). MUST be the bar from the PREVIOUS call,
                  NOT the current bar.
        pdf: PDFDocument instance for signal lookup.

    Returns:
        List of dicts with keys: signal_key, bar_dict, pdf_entry.
        Empty list if no valid signal detected.
    """
    signals = detect_atomic_signals_on_bar(bar_dict, prev_bar, timeframe="ltf")

    if not signals or not pdf:
        return []

    key = "|".join(sorted(signals))
    pdf_entry = pdf.get_signal(key)

    if not pdf_entry or not pdf_entry.sufficient_samples:
        return []

    return [{
        "signal_key": key,
        "bar_dict": bar_dict,
        "pdf_entry": pdf_entry,
    }]


if __name__ == "__main__":
    import argparse
    import time
    from datetime import datetime, timezone

    from loguru import logger
    from rpc.playground_twirp import PlaygroundServiceClient
    from rpc.playground_pb2 import WriteSignalRequest
    from google.protobuf.timestamp_pb2 import Timestamp
    from engine.datasource_heartbeat import DatasourceHeartbeat

    parser = argparse.ArgumentParser(description="MA Crossover datasource (standalone)")
    parser.add_argument("--server-url", default="http://localhost:5051")
    parser.add_argument("--symbol", required=True)
    parser.add_argument("--interval", type=int, default=60)
    parser.add_argument("--signal-name", default="MA_CROSSOVER")
    args = parser.parse_args()

    logger.info(
        "Starting MA Crossover datasource | symbol={} server={} interval={}s",
        args.symbol, args.server_url, args.interval,
    )

    heartbeat = DatasourceHeartbeat("ma-crossover", symbol=args.symbol)
    heartbeat.start()

    client = PlaygroundServiceClient(args.server_url, timeout=30)

    try:
        prev_bar = None
        while True:
            # TODO: Wire real live data fetching here (e.g., Polygon REST/WebSocket).
            # In sim mode, bar_dict and pdf come from the backtester tick loop.
            # For now, the standalone loop demonstrates the RPC wiring pattern.
            bar_dict = {}
            pdf = None

            signals = produce_signals(bar_dict, prev_bar, pdf)

            for sig in signals:
                ts = Timestamp()
                ts.FromDatetime(datetime.now(timezone.utc))
                req = WriteSignalRequest(
                    name=args.signal_name,
                    symbol=args.symbol,
                    timestamp=ts,
                    attributes={k: str(v) for k, v in sig.items() if k != "pdf_entry"},
                )
                resp = client.WriteSignal(ctx={}, request=req)
                logger.info("Signal written | id={}", resp.signal_id)

            heartbeat.record_check()
            prev_bar = bar_dict if bar_dict else prev_bar
            time.sleep(args.interval)
    except KeyboardInterrupt:
        logger.info("Shutting down MA Crossover datasource")
        heartbeat.stop()
