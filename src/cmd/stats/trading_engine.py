from loguru import logger
from base_open_strategy import BaseOpenStrategy
from simple_close_strategy import SimpleCloseStrategy
from trading_engine_types import OpenSignal, OpenSignalV2, OpenSignalName
from playground_metrics import collect_data
from backtester_playground_client_grpc import BacktesterPlaygroundClient, OrderSide, RepositorySource, PlaygroundEnvironment, Repository, CreatePolygonPlaygroundRequest
from typing import List, Tuple
from datetime import datetime
import time
import argparse
import os
import sys

# todo:
# refactor open_strategy to parameterize short and long periods

def configure_logger():
    logger.remove()

    logger.add(sys.stdout, filter=lambda record: record["level"].name not in ["DEBUG", "WARNING"])
    
    env = os.getenv("PLAYGROUND_ENV")
    if env == "live":
        level = "TRACE"
    else:
        level = "INFO"

    # Add a console sink
    logger.add(
        sink=lambda msg: print(msg, end=""),  # Print to console
        format="{time:YYYY-MM-DD HH:mm:ss} | {level} | {message}",
        level=level
    )
    
    s = os.getenv("SYMBOL")

    # Add a file sink
    logger.add(
        f"logs/app-{s}-{datetime.now().strftime('%Y-%m-%d-%H-%M-%S')}.log",  # Log file name
        rotation="10 MB",  # Rotate when file size reaches 10MB
        retention="7 days",  # Keep logs for 7 days
        level=level,
        format="{time:YYYY-MM-DD HH:mm:ss} | {level} | {message}"
    )

def get_sl_tp(signal: OpenSignal) -> Tuple[float, float]:
    sl = signal.min_price_prediction
    tp = signal.max_price_prediction
    return sl, tp

def calculate_required_margin(price: float, qty: float, side: OrderSide) -> float:
    if side == OrderSide.BUY:
        return qty * price * 0.5
    elif side == OrderSide.SELL_SHORT:
        if price < 5:
            calc_price = max(price, 2.5)
            return abs(qty) * calc_price
        
        return abs(qty) * price * 1.5
    else:
        raise ValueError("Invalid side")
        

def calculate_new_trade_quantity(equity: float, free_margin: float, current_price: float, side: OrderSide, stop_loss: float, max_per_trade_risk_percentage: float, max_allowable_free_margin_percentage: float, additional_equity_at_risk: float) -> float:
    max_allowable_margin = free_margin * max_allowable_free_margin_percentage
    max_per_trade_risk = equity * max_per_trade_risk_percentage
    max_per_trade_risk += additional_equity_at_risk
    
    sl_distance = abs(current_price - stop_loss)
    quantity = max_per_trade_risk / sl_distance

    required_margin = calculate_required_margin(current_price, quantity, side)
    if required_margin > max_allowable_margin:
        _quantity = max_allowable_margin / calculate_required_margin(current_price, 1, side)
        logger.debug(f"reducing quantity {quantity:.2f} -> {_quantity:.2f}: required_margin of {required_margin:.2f} > max_allowable_margin of {max_allowable_margin:.2f}")
        quantity = _quantity
        
    # round stock quantity to nearest whole number
    quantity = int(round(quantity - 0.5, 0))
    if quantity < 1:
        raise ValueError(f"Invalid quantity: {quantity}")
    
    return quantity

def build_tag(sl: float, tp: float, side: OrderSide) -> str:
    """
        Builds a tag on the order in the format sl--{sl}--tp--{tp}, e.g. sl--100-50--tp--200-00
    """
        
    if side == OrderSide.BUY or side == OrderSide.SELL_SHORT:   
        sl_str = str(round(sl, 2)).replace('.', '-')
        tp_str = str(round(tp, 2)).replace('.', '-')
    else:
        raise ValueError("Invalid side")
    
    return f"sl--{sl_str}--tp--{tp_str}"

def calculate_sl_tp(side: OrderSide, current_price: float, min_value: float, max_value: float, sl_shift: float, tp_shift: float, sl_buffer: float, tp_buffer: float) -> Tuple[float, float]:
    """ Builds a tag for the order based on the current price and the min and max values.
        min_value and max_value are the min and max values of the price prediction.
        min_value_sd and max_value_sd are the standard deviations of the min and max values.
    """
    if not current_price:
        raise ValueError("current_price not found")
        
    if side == OrderSide.BUY:
        tp_target = max_value + tp_shift
        if tp_target < current_price + tp_buffer:
            raise ValueError(f"[OrderSide.BUY] Invalid target price: tp_target of {tp_target} < {current_price + tp_buffer} = {current_price} + {tp_buffer}")
        
        sl_target = min_value - sl_shift
        if sl_target > current_price - sl_buffer:
            raise ValueError(f"[OrderSide.BUY] Invalid target price: sl_target of {sl_target} > {current_price - sl_buffer} = {current_price} - {sl_buffer}")
        
    elif side == OrderSide.SELL_SHORT:
        tp_target = min_value - tp_shift
        if tp_target > current_price - tp_buffer:
            raise ValueError(f"[OrderSide.SELL_SHORT] Invalid target price: tp_target of {tp_target} > {current_price - tp_buffer} = {current_price} - {tp_buffer}")
        
        sl_target = max_value + sl_shift
        if sl_target < current_price + sl_buffer:
            raise ValueError(f"[OrderSide.SELL_SHORT] Invalid target price: sl_target of {sl_target} < {current_price + sl_buffer} = {current_price} + {sl_buffer}")
        
    else:
        raise ValueError("Invalid side")
        
    return sl_target, tp_target

def run_strategy(symbol, playground, ltf_period, playground_tick_in_seconds, initial_balance, open_strategy: BaseOpenStrategy, close_strategy, grpc_host) -> Tuple[float, dict]:
    sl_shift = open_strategy.get_sl_shift()
    tp_shift = open_strategy.get_tp_shift()
    sl_buffer = open_strategy.get_sl_buffer()
    tp_buffer = open_strategy.get_tp_buffer()
    
    while not open_strategy.is_complete():
        try:
            current_price = playground.get_current_candle(symbol, period=ltf_period).close
        except Exception as e:
            current_price = None
            logger.debug(f"warn: failed to get current price: {e}")
            
        # check for close signals
        close_signals = close_strategy.tick(current_price)
        for s in close_signals:
            resp = playground.place_order(s.Symbol, s.Volume, s.Side, current_price, s.Reason, raise_exception=True, with_tick=True)
            logger.info(f"Placed close order: {resp}")

        # check for open signals
        tick_delta = playground.flush_new_state_buffer()
        for event in tick_delta:
            for trade in event.new_trades:
                logger.info('-' * 40)
                logger.info(f"New {trade.symbol} trade found: {trade.quantity} @ {trade.price} on {trade.create_date}")
                logger.info('-' * 40)
        
        signals = open_strategy.tick(tick_delta)
        position = None
        if len(signals) > 0:
            pos = playground.account.get_position(symbol)
            position = pos.quantity if pos else 0
                
        if len(signals) > 1:
            logger.error(f"Multiple signals detected: {signals}")
            
        for s in signals:            
            if s.name == OpenSignalName.CROSS_ABOVE_20:
                if position < 0:
                    qty = abs(position)
                    side = OrderSide.BUY_TO_COVER
                    resp = playground.place_order(symbol, qty, side, current_price, 'close-all', raise_exception=True, with_tick=True)
                    logger.info(f"Placed close order: {resp}")

                side = OrderSide.BUY
            elif s.name == OpenSignalName.CROSS_BELOW_80:
                if position > 0:
                    qty = position
                    side = OrderSide.SELL
                    resp = playground.place_order(symbol, qty, side, current_price, 'close-all', raise_exception=True, with_tick=True)
                    logger.info(f"Placed close order: {resp}") 
                    
                side = OrderSide.SELL_SHORT
            else:
                logger.error(f"Unknown signal: {s.name}")
                continue
            
            try:
                sl, tp = calculate_sl_tp(side, current_price, s.min_price_prediction, s.max_price_prediction, sl_shift, tp_shift, sl_buffer, tp_buffer)
                additional_equity_at_risk = 0
                if isinstance(s, OpenSignalV2):
                    additional_equity_at_risk = s.additional_equity_risk
                
                max_per_trade_risk_percentage = 0.06
                max_allowable_free_margin_percentage = 0.65
                quantity = calculate_new_trade_quantity(playground.account.equity, playground.account.free_margin, current_price, side, s.min_price_prediction, max_per_trade_risk_percentage, max_allowable_free_margin_percentage, additional_equity_at_risk)
                tag = build_tag(sl, tp, side)
            except ValueError as e:
                logger.warning(f"failed to build tag: {e}. Skipping order ...")
                continue
            
            try:
                resp = playground.place_order(symbol, quantity, side, current_price, tag)
            except Exception as e:
                logger.error(f"Error placing order: {e}")
                continue
            
            logger.info(f"Placed open order: {resp}")
            
        playground.tick(playground_tick_in_seconds, raise_exception=True)
        
    profit = playground.account.equity - initial_balance
    logger.info(f"Playground: {playground.id} completed with profit of {profit:.2f} and (sl_shift, tp_shift, sl_buffer, tp_buffer) of ({sl_shift}, {tp_shift}, {sl_buffer}, {tp_buffer})")
    
    # fetch stats
    stats = collect_data(grpc_host, playground.id)
    
    meta = {
        'profit': profit,
        'sl_shift': sl_shift,
        'tp_shift': tp_shift,
        'sl_buffer': sl_buffer,
        'tp_buffer': tp_buffer,
        'equity': playground.account.equity,
        'stats': stats
    }
    
    playground.remove_from_server()
    
    logger.info(f"Removed playground: {playground.id}")
    
    return profit, meta
    
def objective(sl_shift = 0.0, tp_shift = 0.0, sl_buffer = 0.0, tp_buffer = 0.0, min_max_window_in_hours=4) -> Tuple[float, dict]:
    # input parameters
    # Read environment variables
    balance = float(os.getenv("BALANCE"))
    symbol = os.getenv("SYMBOL")
    grpc_host = os.getenv("GRPC_HOST")
    playground_env = os.getenv("PLAYGROUND_ENV")
    live_account_type = os.getenv("LIVE_ACCOUNT_TYPE")
    open_strategy_input = os.getenv("OPEN_STRATEGY")
    start_date = os.getenv("START_DATE")
    stop_date = os.getenv("STOP_DATE")
    model_update_frequency = os.getenv("MODEL_UPDATE_FREQUENCY")
    optimizer_update_frequency = os.getenv("OPTIMIZER_UPDATE_FREQUENCY")
    n_calls = os.getenv("N_CALLS")
    playground_client_id = os.getenv("PLAYGROUND_CLIENT_ID")

    # Check if the required environment variables are set
    if balance is None:
        raise ValueError("Environment variable BALANCE is not set")
    if symbol is None:
        raise ValueError("Environment variable SYMBOL is not set")
    if grpc_host is None:
        raise ValueError("Environment variable GRPC_HOST is not set")
    if playground_env is None:
        raise ValueError("Environment variable PLAYGROUND_ENV is not set")
    if playground_env.lower() == "live":
        if live_account_type is None:
            raise ValueError("Environment variable LIVE_ACCOUNT_TYPE is not set")
    else:
        if start_date is None:
            raise ValueError("Environment variable START_DATE is not set")
        if stop_date is None:
            raise ValueError("Environment variable STOP_DATE is not set")
        
    if model_update_frequency is None:
        raise ValueError("Environment variable MODEL_UPDATE_FREQUENCY is not set")

    if optimizer_update_frequency is None:
        raise ValueError("Environment variable OPTIMIZER_UPDATE_FREQUENCY is not set")
    
    if live_account_type is not None:
        logger.info(f'starting {playground_env} playgound for {symbol} with account type {live_account_type}')
    else:
        logger.info(f'starting {playground_env} playgound for {symbol}')
        
    if n_calls is None:
        raise ValueError("Environment variable N_CALLS is not set")
    n_calls = int(n_calls)
    
    if playground_env.lower() == "simulator":
        playground_tick_in_seconds = 300
        start_date = start_date
        stop_date = stop_date
        repository_source = RepositorySource.POLYGON
        env = PlaygroundEnvironment.SIMULATOR
    
        logger.info(f"initializing {env}: {symbol} playground from {start_date} to {stop_date} ...")
        
    elif playground_env.lower() == "live":
        playground_tick_in_seconds = 20  # too fast until we update position with pending orders that have not yet filled at broker
        start_date = None
        stop_date = None
        repository_source = None
        env = PlaygroundEnvironment.LIVE
        
        logger.info(f"initializing {env}: {symbol} playground")
        
    else:
        raise ValueError(f"Invalid environment: {playground_env}")
    
    ltf_repo = Repository(
        symbol=symbol,
        timespan_multiplier=5,
        timespan_unit='minute',
        indicators=["supertrend", "stochrsi", "moving_averages", "lag_features", "atr", "stochrsi_cross_above_20", "stochrsi_cross_below_80"],
        history_in_days=365
    )
        
    ltf_period = ltf_repo.timespan_multiplier * 60
    
    htf_repo = Repository(
        symbol=symbol,
        timespan_multiplier=60,
        timespan_unit='minute',
        indicators=["supertrend"],
        history_in_days=365
    )
    
    req = CreatePolygonPlaygroundRequest(
        balance=balance,
        start_date=start_date,
        stop_date=stop_date,
        repositories=[ltf_repo, htf_repo],
        environment=env.value
    )
    
    if playground_client_id is not None:
        req.client_id = playground_client_id
        logger.info(f"using existing playground with id: {playground_client_id}")
    
    playground = BacktesterPlaygroundClient(req, live_account_type, repository_source, grpc_host=grpc_host)
    
    playground.tick(0, raise_exception=True)  # initialize the playground
    
    logger.info(f"created playground with id: {playground.id}")
    
    if open_strategy_input == 'simple_open_strategy_v1':
        from simple_open_strategy_v1 import SimpleOpenStrategy
        open_strategy = SimpleOpenStrategy(playground, model_update_frequency, sl_shift, tp_shift, sl_buffer, tp_buffer, min_max_window_in_hours)
        
    elif open_strategy_input == 'simple_open_strategy_v2':
        from simple_open_strategy_v2 import OptimizedOpenStrategy
        open_strategy = OptimizedOpenStrategy(playground, model_update_frequency, optimizer_update_frequency, n_calls)
        
    elif open_strategy_input == 'simple_open_strategy_v3':
        from simple_open_strategy_v3 import SimpleOpenStrategyV3
        additional_profit_risk_percentage = 0.25
        open_strategy = SimpleOpenStrategyV3(playground, additional_profit_risk_percentage, model_update_frequency, sl_shift, tp_shift, sl_buffer, tp_buffer, min_max_window_in_hours)
        
    else:
        logger.error(f"Invalid open strategy: {open_strategy_input}")
        raise ValueError(f"Invalid open strategy: {open_strategy_input}")
    
    close_strategy = SimpleCloseStrategy(playground)
    
    return run_strategy(symbol, playground, ltf_period, playground_tick_in_seconds, balance, open_strategy, close_strategy, grpc_host)

if __name__ == "__main__":
    args = argparse.ArgumentParser()
    args.add_argument("--sl-shift", type=float, default=0.0)
    args.add_argument("--tp-shift", type=float, default=0.0)
    args.add_argument("--sl-buffer", type=float, default=0.0)
    args.add_argument("--tp-buffer", type=float, default=0.0)
    args.add_argument("--min-max-window-in-hours", type=int, default=4)
    args = args.parse_args()
    
    configure_logger()
    
    logger.info(f"starting trading engine with inputs sl_shift: {args.sl_shift}, tp_shift: {args.tp_shift}, sl_buffer: {args.sl_buffer}, tp_buffer: {args.tp_buffer}, min_max_window_in_hours: {args.min_max_window_in_hours}")
    
    profit, meta = objective(args.sl_shift, args.tp_shift, args.sl_buffer, args.tp_buffer, args.min_max_window_in_hours)
    
    logger.info(f"profit: {profit}, meta: {meta}")
    