from loguru import logger
from base_open_strategy import BaseOpenStrategy
from simple_close_strategy import SimpleCloseStrategy
from simple_stack_close_strategy import SimpleStackCloseStrategy
from trading_engine_types import OpenSignal, OpenSignalV2, OpenSignalV3, OpenSignalName
from playground_metrics import collect_data
from rpc.playground_twirp import PlaygroundServiceClient
from backtester_playground_client_grpc import BacktesterPlaygroundClient, OrderSide, RepositorySource, PlaygroundEnvironment, Repository, CreatePolygonPlaygroundRequest, InvalidParametersException, PlaceOrderSideNotAllowedException
from typing import List, Tuple
from datetime import datetime, timedelta
from scipy.stats import t
from utils import get_timespan_unit
import json
import numpy as np
import time
import argparse
import os
import sys

# todo:
# refactor open_strategy to parameterize short and long periods
logger.remove()

logger.add(sys.stdout, filter=lambda record: record["level"].name not in ["DEBUG", "WARNING"])

# Configure loguru to log to both console and file
logger = logger.bind(timestamp="", trading_operation="")
logger.add("trading_engine_{time}.log", format="timestamp={extra[timestamp]} trading_operation={extra[trading_operation]} {message}", rotation="1 day", retention="14 days", level="INFO")

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
    f"logs/app-{s}-{datetime.now().strftime('%Y-%m-%d.%H-%M-%S')}.log",  # Log file name
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
        

def calculate_new_trade_quantity(logger, equity: float, free_margin: float, current_price: float, side: OrderSide, stop_loss: float, max_per_trade_risk_percentage: float, max_allowable_free_margin_percentage: float, additional_equity_at_risk: float) -> float:
    max_allowable_margin = free_margin * max_allowable_free_margin_percentage
    max_per_trade_risk = equity * max_per_trade_risk_percentage
    
    logger.info(f"max_allowable_margin: {max_allowable_margin:.2f}, free_margin: {free_margin}, max_allowable_free_margin_percentage: {max_allowable_free_margin_percentage:.2f}", trading_operation='calculate_risk')
    logger.info(f"original max_per_trade_risk: {max_per_trade_risk:.2f}, equity: {equity:.2f}, max_per_trade_risk_percentage: {max_per_trade_risk_percentage:.2f}", trading_operation='calculate_risk')
    
    max_per_trade_risk += additional_equity_at_risk
    
    sl_distance = abs(current_price - stop_loss)
    
    logger.info(f"current_price: {current_price}, stop_loss: {stop_loss}, sl_distance: {sl_distance}", trading_operation='calculate_risk')
    
    quantity = max_per_trade_risk / sl_distance
    
    logger.info(f"new max_per_trade_risk: {max_per_trade_risk}, quantity: {quantity}", trading_operation='calculate_risk')

    required_margin = calculate_required_margin(current_price, quantity, side)
    if required_margin > max_allowable_margin:
        _quantity = max_allowable_margin / calculate_required_margin(current_price, 1, side)
        logger.info(f"reducing quantity {quantity:.2f} -> {_quantity:.2f}: required_margin of {required_margin:.2f} > max_allowable_margin of {max_allowable_margin:.2f}", trading_operation='calculate_risk')
        quantity = _quantity
        
    # round stock quantity to nearest whole number
    quantity = int(round(quantity - 0.5, 0))
    logger.trace(f"calculate_new_trade_quantity: outputting final quantity: {quantity}", trading_operation='calculate_risk')
    if quantity < 1:
        raise ValueError(f"Invalid quantity: {quantity}")
    
    return quantity

def build_client_request_id(symbol: str, date: str, side: OrderSide, quantity: float) -> str:
    """
        Builds a client request id in the format {symbol}--{date}--{side}--{quantity}, e.g. EURUSD--2023-10-01--BUY--1000
    """
    if side == OrderSide.BUY:
        side_str = "buy"
    elif side == OrderSide.SELL:
        side_str = "sell"
    elif side == OrderSide.SELL_SHORT:
        side_str = "sell_short"
    elif side == OrderSide.BUY_TO_COVER:
        side_str = "buy_to_cover"
    else:
        raise ValueError("Invalid side")
    
    return f"{symbol}--{date}--{side_str}--{quantity:.2f}"

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

def calculate_margin_of_error(confidence_level: float, mse: float, n: int) -> float:
    se = np.sqrt(mse / n)
    alpha = 1 - confidence_level
    t_crit = t.ppf(1 - alpha / 2, df=n-1)
    return t_crit * se 

def calculate_sl_tp(side: OrderSide, current_price: float, signal: OpenSignalV2, sl_confidence_weight: float, tp_confidence_weight: float, sl_buffer: float, tp_buffer: float) -> Tuple[float, float]:
    """ Builds a tag for the order based on the current price and the min and max values.
        min_value and max_value are the min and max values of the price prediction.
        min_value_sd and max_value_sd are the standard deviations of the min and max values.
    """
    if not current_price:
        raise ValueError("current_price not found")
        
    min_value = signal.min_price_prediction  
    max_value = signal.max_price_prediction
    
    if side == OrderSide.BUY:
        min_value_margin_of_error = calculate_margin_of_error(0.95, signal.min_price_prediction_mse, signal.min_price_prediction_n)
        lower_bound = min_value - (min_value_margin_of_error * sl_confidence_weight)
        
        if lower_bound < sl_buffer:
            raise ValueError(f"[OrderSide.BUY] Too small: diff(current_price, min_value): {current_price - min_value} < sl_buffer: {sl_buffer}")
        
        sl_target = lower_bound
        
        max_value_margin_of_error = calculate_margin_of_error(0.95, signal.max_price_prediction_mse, signal.max_price_prediction_n)
        upper_bound = max_value + (max_value_margin_of_error * tp_confidence_weight)
        
        if upper_bound < tp_buffer:
            raise ValueError(f"[OrderSide.BUY] Too small: upper_bound: {upper_bound} - current_price: {current_price} < tp_buffer: {tp_buffer}")
        
        tp_target = upper_bound
        
    elif side == OrderSide.SELL_SHORT:
        max_value_margin_of_error = calculate_margin_of_error(0.95, signal.max_price_prediction_mse, signal.max_price_prediction_n)
        upper_bound = max_value + (max_value_margin_of_error * sl_confidence_weight)
        
        if upper_bound < sl_buffer:
            raise ValueError(f"[OrderSide.SELL_SHORT] Too small: upper_bound: {upper_bound} - current_price: {current_price} < sl_buffer: {sl_buffer}")
        
        sl_target = upper_bound
        
        min_value_margin_of_error = calculate_margin_of_error(0.95, signal.min_price_prediction_mse, signal.min_price_prediction_n)
        lower_bound = min_value - (min_value_margin_of_error * tp_confidence_weight)
        
        if lower_bound < tp_buffer:
            raise ValueError(f"[OrderSide.SELL_SHORT] Too small: diff(current_price, lower_bound): {current_price - lower_bound} < tp_buffer: {tp_buffer}")
        
        tp_target = lower_bound
        
    else:
        raise ValueError("Invalid side")
        
    return sl_target, tp_target    

def run_strategy(symbols, playground, ltf_period, playground_tick_in_seconds, initial_balance, open_strategies: List[BaseOpenStrategy], close_strategy, twirp_host) -> Tuple[float, dict]:
    # sl_shift = open_strategy.get_sl_shift()
    # tp_shift = open_strategy.get_tp_shift()
    # sl_buffer = open_strategy.get_sl_buffer()
    # tp_buffer = open_strategy.get_tp_buffer()
    sl_shift = 0.0
    tp_shift = 0.0
    sl_buffer = 0.0
    tp_buffer = 0.0
    
    i = 0
    while not playground.is_backtest_complete():
        try:
            new_candles_dict = {}
            current_prices_dict = {symbol: None for symbol in symbols}
            tick_delta = playground.flush_new_state_buffer()
            for event in tick_delta:
                for trade in event.new_trades:
                    logger.info(f"New Fill: {trade.quantity} @ {trade.price} on {trade.create_date}", timestamp=playground.timestamp, trading_operation='new_trade')
            
                for candle in event.new_candles:
                    if candle.symbol not in new_candles_dict:
                        new_candles_dict[candle.symbol] = []
                    new_candles_dict[candle.symbol].append(candle)
                    
            for open_strategy in open_strategies:
                new_candles = new_candles_dict.get(open_strategy.symbol, [])
                open_strategy.update_price_feed(new_candles)
                new_candles_dict[open_strategy.symbol] = new_candles
             
                current_candle = playground.get_current_candle(open_strategy.symbol, ltf_period)
                current_prices_dict[open_strategy.symbol] = current_candle.close
                
        except Exception as e:
            new_candles_dict = {} 
            current_prices_dict = {symbol: None for symbol in symbols}
            logger.debug(f"warn: failed to update price feed: {e}")
            time.sleep(1)
            continue
        
        i += 1   
        # check for close signals
        kwargs = {
            'period': ltf_period,
            'playground': playground,
        }
        
        if isinstance(close_strategy, SimpleStackCloseStrategy):
            kwargs['supertrend_direction'] = current_candle.superD_50_3
            kwargs['tp_buffer'] = tp_buffer
            max_per_trade_risk_percentage = open_strategy.get_max_per_trade_risk_percentage()
        else:
            max_per_trade_risk_percentage = 0.06
            
        
        close_signals = []
        for symbol in symbols:
            prc = current_prices_dict[symbol]
            close_signals.extend(
                close_strategy.tick(symbol, prc, kwargs)
            )
            
        for s in close_signals:
            if playground.timestamp - s.Timestamp > timedelta(minutes=5):
                logger.warning(f"Ignoring close signal: {s.Timestamp} - {s.Symbol} - {s.Side} - {s.Volume} - Diff: {playground.timestamp - s.Timestamp}", timestamp=playground.timestamp, trading_operation='close')
                continue
            else:
                logger.info(f"playground (tstamp): {playground.timestamp} - close signal (tstamp): {s.Timestamp} - Diff: {playground.timestamp - s.Timestamp}", timestamp=playground.timestamp, trading_operation='close')
            
            client_id = build_client_request_id(s.Symbol, s.Timestamp.strftime("%Y-%m-%d.%H:%M:%S"), s.Side, s.Volume)
            
            try:
                price = current_prices_dict[s.Symbol]
                resp = playground.place_order(s.Symbol, s.Volume, s.Side, price, s.Reason, close_order_id=s.OrderId, raise_exception=True, with_tick=True, sl=None, client_request_id=client_id)
                logger.info(f"Placed close order: ({resp.id}, {resp.external_id})", timestamp=playground.timestamp, trading_operation='close')
            except Exception as e:
                logger.error(f"Error placing close order: {e}", timestamp=playground.timestamp, trading_operation='close')
                continue

        # check for open signals        
        for open_strategy in open_strategies:
            new_candles = new_candles_dict[open_strategy.symbol]
            open_signals = open_strategy.tick(new_candles)
            position = None
            if len(open_signals) > 0:
                pos = playground.account.get_position(open_strategy.symbol)
                position = pos.quantity if pos else 0
                    
            if len(open_signals) > 1:
                logger.error(f"Multiple signals detected: {open_signals}")
                
            for s in open_signals:
                symbol = s.symbol
                ts = s.timestamp
                
                if symbol != open_strategy.symbol:
                    logger.error(f"Signal symbol {symbol} does not match open strategy symbol {open_strategy.symbol}", timestamp=playground.timestamp, trading_operation='process_open_signal')
                    continue
                
                if type(ts) is not datetime:
                    ts = ts.to_pydatetime()
                    
                if playground.timestamp - ts > timedelta(minutes=10):
                    logger.warning(f"Ignoring open signal: diff - {playground.timestamp - s.timestamp} > 10: {s.timestamp} - {symbol} - {side} - {qty}", timestamp=playground.timestamp, trading_operation='process_open_signal')
                    continue
                else:
                    logger.info(f"playground (tstamp): {playground.timestamp} - open signal (tstamp): {s.timestamp} - Diff: {playground.timestamp - ts}", timestamp=playground.timestamp, trading_operation='process_open_signal')
                
                if s.name == OpenSignalName.SUPERTREND_STACK_SIGNAL:
                    side = s.kwargs['side']
                    if position > 0 and side == OrderSide.SELL_SHORT:
                        qty = position
                        client_id = build_client_request_id(symbol, s.timestamp.strftime("%Y-%m-%d.%H:%M:%S"), OrderSide.SELL, qty)
                        current_price = current_prices_dict[symbol]
                        resp = playground.place_order(symbol, qty, OrderSide.SELL, current_price, 'close-all', raise_exception=True, with_tick=True, sl=None, client_request_id=client_id)
                        logger.info(f"Placed close all order: SUPERTREND_STACK_SIGNAL - {resp.id}", timestamp=playground.timestamp, trading_operation='close_long')
                        
                    elif position < 0 and side == OrderSide.BUY:
                        qty = abs(position)
                        client_id = build_client_request_id(symbol, s.timestamp.strftime("%Y-%m-%d.%H:%M:%S"), OrderSide.BUY_TO_COVER, qty)
                        current_price = current_prices_dict[symbol]
                        resp = playground.place_order(symbol, qty, OrderSide.BUY_TO_COVER, current_price, 'close-all', raise_exception=True, with_tick=True, sl=None, client_request_id=client_id)
                        logger.info(f"Placed close all order: SUPERTREND_STACK_SIGNAL - {resp.id}", timestamp=playground.timestamp, trading_operation='close_short')
                        
                elif s.name == OpenSignalName.CROSS_ABOVE_20:
                    if position < 0:
                        qty = abs(position)
                        side = OrderSide.BUY_TO_COVER
                        client_id = build_client_request_id(symbol, s.timestamp.strftime("%Y-%m-%d.%H:%M:%S"), side, qty)
                        current_price = current_prices_dict[symbol]
                        resp = playground.place_order(symbol, qty, side, current_price, 'close-all', raise_exception=True, with_tick=True, sl=None, client_request_id=client_id)
                        logger.info(f"Placed close all order: CROSS_ABOVE_20 - {resp.id}", timestamp=playground.timestamp, trading_operation='close_short')

                    side = OrderSide.BUY
                elif s.name == OpenSignalName.CROSS_BELOW_80:
                    if position > 0:
                        qty = position
                        side = OrderSide.SELL
                        client_id = build_client_request_id(symbol, s.timestamp.strftime("%Y-%m-%d.%H:%M:%S"), side, qty)
                        current_price = current_prices_dict[symbol]
                        resp = playground.place_order(symbol, qty, side, current_price, 'close-all', raise_exception=True, with_tick=True, sl=None, client_request_id=client_id)
                        logger.info(f"Placed close all order: CROSS_BELOW_80 - {resp.id}", timestamp=playground.timestamp, trading_operation='close_long')
                        
                    side = OrderSide.SELL_SHORT
                else:
                    logger.error(f"Unknown signal: {s.name}", timestamp=playground.timestamp, trading_operation='open')
                    continue
                
                try:
                    additional_equity_at_risk = 0
                    max_allowable_free_margin_percentage = 0.65
                    current_price = current_prices_dict[symbol]
                    
                    if isinstance(s, OpenSignalV3):
                        count = s.kwargs['count']
                        tag = f'SupertrendStackSignal-{count}'
                        sl = s.kwargs['sl']
                    else:
                        sl, tp = calculate_sl_tp(side, current_price, s, sl_shift, tp_shift, sl_buffer, tp_buffer)
                        logger.info(f"calculated sl: {sl}, tp: {tp}", timestamp=playground.timestamp, trading_operation='open')
                        if isinstance(s, OpenSignalV2):
                            additional_equity_at_risk = s.additional_equity_risk
                    
                        tag = build_tag(sl, tp, side)
                
                    quantity = calculate_new_trade_quantity(logger, playground.account.equity, playground.account.free_margin, current_price, side, sl, max_per_trade_risk_percentage, max_allowable_free_margin_percentage, additional_equity_at_risk)
                        
                except ValueError as e:
                    logger.warning(f"failed to calculate order quantity: {e}. Skipping order ...", timestamp=playground.timestamp, trading_operation='error')
                    continue
                
                try:
                    attempt = 0
                    client_id = build_client_request_id(symbol, s.timestamp.strftime("%Y-%m-%d.%H:%M:%S"), side, quantity)
                    resp = playground.place_order(symbol, quantity, side, current_price, tag, sl=sl, client_request_id=client_id)
                except InvalidParametersException as e:
                    logger.warning(f"Error placing order: {e}", timestamp=playground.timestamp, trading_operation='open')
                    continue
                except PlaceOrderSideNotAllowedException as e:
                    logger.warning(f"Error placing order: {e}. Skipping order ...", timestamp=playground.timestamp, trading_operation='open')
                    continue
                except Exception as e:
                    logger.error(f"Error placing order: {e}", timestamp=playground.timestamp, trading_operation='open')
                    continue
                
                logger.info(f"Placed open order: {resp.id}", timestamp=playground.timestamp, trading_operation='open')
            
        # tick the playground
        playground.tick(playground_tick_in_seconds, raise_exception=True)
        
    profit = playground.account.equity - initial_balance
    logger.info(f"Playground: {playground.id} completed with profit of {profit:.2f} and (sl_shift, tp_shift, sl_buffer, tp_buffer) of ({sl_shift}, {tp_shift}, {sl_buffer}, {tp_buffer})")
    
    # fetch stats
    # orders = playground.fetch_orders()
    # position = playground.account.get_position(symbol)
    # from_date = playground.account.meta.start_date
    
    # stats = collect_data(orders, position, from_date)
    
    meta = {
        'profit': profit,
        'sl_shift': sl_shift,
        'tp_shift': tp_shift,
        'sl_buffer': sl_buffer,
        'tp_buffer': tp_buffer,
        'equity': playground.account.equity,
        'stats': None
    }
    
    playground.remove_from_server()
    
    logger.info(f"Removed playground: {playground.id}")
    
    return profit, meta

def build_client_version_tag(client_id: str) -> str:
    """
        Builds a client version tag in the format cli_v{client_id}
    """
    if client_id is None:
        raise ValueError("client_id is None")
    
    idx = client_id.rfind('-')
    if idx == -1:
        raise ValueError("client_id is invalid")
    
    version = client_id[idx+1:]
    if version is None:
        raise ValueError("version is None")
    
    return f"cli_v{version}"

def parse_symbols(symbols: str) -> List[str]:
    """
        Parses a comma-separated string of symbols into a list.
        e.g. "AAPL,GOOGL,MSFT" -> ["AAPL", "GOOGL", "MSFT"]
    """
    if not symbols:
        raise ValueError("symbols is empty")
    
    return [s.strip() for s in symbols.split(' ') if s.strip()]
    
def objective(logger, kwargs) -> Tuple[float, dict]:
    if kwargs is None:
        raise ValueError("kwargs is None")
    
    if type(kwargs) is not dict:
        raise ValueError("kwargs is not a dict")
    
    # default parameters
    sl_shift = kwargs.get('sl_shift', 0.0)
    tp_shift = kwargs.get('tp_shift', 0.0)
    sl_buffer = kwargs.get('sl_buffer', 0.0)
    tp_buffer = kwargs.get('tp_buffer', 0.0)
    min_max_window_in_hours = kwargs.get('min_max_window_in_hours', 4)
    
    # input parameters
    # Read environment variables
    balance = float(os.getenv("BALANCE"))
    symbols = parse_symbols(os.getenv("SYMBOL")) # TODO: Change to SYMBOLS
    twirp_host = os.getenv("TWIRP_HOST")
    playground_env = os.getenv("PLAYGROUND_ENV")
    live_account_type = os.getenv("LIVE_ACCOUNT_TYPE")
    open_strategy_input = os.getenv("OPEN_STRATEGY")
    start_date = os.getenv("START_DATE")
    stop_date = os.getenv("STOP_DATE")
    # optimizer_update_frequency = os.getenv("OPTIMIZER_UPDATE_FREQUENCY")
    playground_client_id = os.getenv("PLAYGROUND_CLIENT_ID")

    # Check if the required environment variables are set
    if balance is None:
        raise ValueError("Environment variable BALANCE is not set")
    if symbols is None:
        raise ValueError("Environment variable SYMBOL is not set")
    if twirp_host is None:
        raise ValueError("Environment variable TWIRP HOST is not set")
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
        
    # if optimizer_update_frequency is None:
    #     raise ValueError("Environment variable OPTIMIZER_UPDATE_FREQUENCY is not set")
    
    if live_account_type is not None:
        logger.info(f'starting {playground_env} playgound for {symbols} with account type {live_account_type}')
    else:
        logger.info(f'starting {playground_env} playgound for {symbols}')
    
    if playground_env.lower() == "simulator":
        playground_tick_in_seconds = 300
        start_date = start_date
        stop_date = stop_date
        repository_source = RepositorySource.POLYGON
        env = PlaygroundEnvironment.SIMULATOR
    
        logger.info(f"initializing {env}: {symbols} playground from {start_date} to {stop_date} ...")
        
    elif playground_env.lower() == "live":
        playground_tick_in_seconds = 20  # too fast until we update position with pending orders that have not yet filled at broker
        start_date = None
        stop_date = None
        repository_source = None
        env = PlaygroundEnvironment.LIVE
        
        logger.info(f"initializing {env}: {symbols} playground")
        
    else:
        raise ValueError(f"Invalid environment: {playground_env}")
    
    
    ltf_repo_timespan_unit = 'minute'
    ltf_repo_timespan_mutliplier = 5
    ltf_period = ltf_repo_timespan_mutliplier * get_timespan_unit(ltf_repo_timespan_unit)
    
    if open_strategy_input == 'simple_stack_open_strategy_v2':
        from simple_stack_open_strategy_v2 import SimpleStackOpenStrategyV2
        
        repos = []
        for s in symbols:
            repos.extend(SimpleStackOpenStrategyV2.get_repositories(s, start_date, stop_date))
            
        tags = [s.lower() for s in symbols]
        tags.append(open_strategy_input)
        
        req = CreatePolygonPlaygroundRequest(
            balance=balance,
            start_date=start_date,
            stop_date=stop_date,
            repositories=repos,
            environment=env.value,
            tags=tags
        )
        
    else:        
        repos = []
        if open_strategy_input == 'candlestick_open_strategy_v1':
            for symbol in symbols:
                repos.append(Repository(
                    symbol=symbol,
                    timespan_multiplier=ltf_repo_timespan_mutliplier,
                    timespan_unit=ltf_repo_timespan_unit,
                    indicators=["supertrend", "doji", "hammer"],
                    history_in_days=365
                ))
        else:
            for symbol in symbols:
                repos.append(Repository(
                    symbol=symbol,
                    timespan_multiplier=ltf_repo_timespan_mutliplier,
                    timespan_unit=ltf_repo_timespan_unit,
                    indicators=["supertrend", "stochrsi", "moving_averages", "lag_features", "atr", "stochrsi_cross_above_20", "stochrsi_cross_below_80"],
                    history_in_days=365
                ))
                    
        htf_repo_timespan_mutliplier = 1
        htf_repo_timespan_unit = 'hour'
        for symbol in symbols:
            repos.append(
                Repository(
                    symbol=symbol,
                    timespan_multiplier=htf_repo_timespan_mutliplier,
                    timespan_unit=htf_repo_timespan_unit,
                    indicators=["supertrend"],
                    history_in_days=365
                )
            )
        
        req = CreatePolygonPlaygroundRequest(
            balance=balance,
            start_date=start_date,
            stop_date=stop_date,
            repositories=repos,
            environment=env.value,
            tags=[symbol.lower(), open_strategy_input]
        )
        
    if playground_client_id is not None:
        req.client_id = playground_client_id
        req.tags.append(build_client_version_tag(playground_client_id))
        logger.info(f"using playground client id: {playground_client_id}")
    
    playground = BacktesterPlaygroundClient(req, live_account_type, repository_source, logger, twirp_host=twirp_host)
        
    playground.tick(0, raise_exception=True)  # initialize the playground
            
    logger.info(f"created playground with id: {playground.id}")
    
    open_strategies = []
    
    for symbol in symbols:
        # TODO: this ideally would happen inside BacktesterPlaygroundClient
        current_bar = playground.fetch_most_recent_bar(symbol, ltf_period, playground.timestamp)
        playground.set_current_candle(symbol, ltf_period, current_bar)
        
        if open_strategy_input == 'simple_open_strategy_v1':
            raise NotImplementedError("simple_open_strategy_v1 is not implemented yet")
            # from simple_open_strategy_v1 import SimpleOpenStrategy
            # model_update_frequency = os.getenv("MODEL_UPDATE_FREQUENCY")
            
            # open_strategy = SimpleOpenStrategy(playground, model_update_frequency, sl_shift, tp_shift, sl_buffer, tp_buffer, min_max_window_in_hours)
            
        elif open_strategy_input == 'simple_open_strategy_v2':
            raise NotImplementedError("simple_open_strategy_v2 is not implemented yet")
            # from simple_open_strategy_v2 import SimpleOptimizedOpenStrategy
            # optimizer_update_frequency = None
            # model_update_frequency = os.getenv("MODEL_UPDATE_FREQUENCY")
            
            # n_calls = os.getenv("N_CALLS")
            # if n_calls is None:
            #     raise ValueError("Environment variable N_CALLS is not set")
            # n_calls = int(n_calls)
            
            # open_strategy = SimpleOptimizedOpenStrategy(playground, model_update_frequency, optimizer_update_frequency, n_calls)
            
        elif open_strategy_input == 'simple_open_strategy_v3':
            raise NotImplementedError("simple_open_strategy_v3 is not implemented yet")
            # from simple_open_strategy_v3 import SimpleOpenStrategyV3
            # additional_profit_risk_percentage = 0.25
            # model_update_frequency = os.getenv("MODEL_UPDATE_FREQUENCY")
            
            # open_strategy = SimpleOpenStrategyV3(playground, additional_profit_risk_percentage, model_update_frequency, sl_shift, tp_shift, sl_buffer, tp_buffer, min_max_window_in_hours)
            
        elif open_strategy_input == 'simple_open_strategy_v4':
            from simple_open_strategy_v4 import SimpleOpenStrategyV4
            additional_profit_risk_percentage = 0.0
            model_update_frequency = os.getenv("MODEL_UPDATE_FREQUENCY")
            
            open_strategies.append(
                SimpleOpenStrategyV4(playground, additional_profit_risk_percentage, model_update_frequency, symbol, logger, sl_shift, tp_shift, sl_buffer, tp_buffer, min_max_window_in_hours)
            )
        
        elif open_strategy_input == 'candlestick_open_strategy_v1':
            raise NotImplementedError("candlestick_open_strategy_v1 is not implemented yet")
            # from candlestick_open_strategy_v1 import CandlestickOpenStrategy
            # min_max_window_in_hours = 24
            # model_update_frequency = os.getenv("MODEL_UPDATE_FREQUENCY")
            
            # open_strategies.append(
            #     CandlestickOpenStrategy(playground, model_update_frequency, min_max_window_in_hours)
            # )
        
        elif open_strategy_input == 'simple_stack_open_strategy_v1':
            from simple_stack_open_strategy_v1 import SimpleStackOpenStrategyV1
            additional_profit_risk_percentage = 0.0
            max_open_count = int(kwargs['max_open_count'])
            target_risk_to_reward = float(kwargs['target_risk_to_reward'])
            max_per_trade_risk_percentage = float(kwargs['max_per_trade_risk_percentage'])
            use_htf_data = kwargs.get('use_htf_data', False)
            
            open_strategies.append(
                SimpleStackOpenStrategyV1(playground, max_open_count, max_per_trade_risk_percentage, additional_profit_risk_percentage, symbol, logger, sl_buffer, tp_buffer, use_htf_data=use_htf_data)
            )
            
        elif open_strategy_input == 'simple_stack_open_strategy_v2':
            from simple_stack_open_strategy_v2 import SimpleStackOpenStrategyV2
            additional_profit_risk_percentage = 0.0
            max_open_count = int(kwargs['max_open_count'])
            target_risk_to_reward = float(kwargs['target_risk_to_reward'])
            max_per_trade_risk_percentage = float(kwargs['max_per_trade_risk_percentage'])
            
            open_strategies.append(
                SimpleStackOpenStrategyV2(playground, max_open_count, max_per_trade_risk_percentage, additional_profit_risk_percentage, symbol, logger, sl_buffer, tp_buffer)
            )

        else:
            logger.error(f"Invalid open strategy: {open_strategy_input}")
            raise ValueError(f"Invalid open strategy: {open_strategy_input}")
        
    if open_strategy_input == 'simple_stack_open_strategy_v1':
        close_strategy = SimpleStackCloseStrategy(playground, logger, max_open_count, target_risk_to_reward)
    else:
        close_strategy = SimpleCloseStrategy(playground, {})
    
    return run_strategy(symbols, playground, ltf_period, playground_tick_in_seconds, balance, open_strategies, close_strategy, twirp_host)

if __name__ == "__main__":
    args = argparse.ArgumentParser()
    args.add_argument("--max-open-count", type=float, default=None)
    args.add_argument("--target-risk-to-reward", type=float, default=None)
    args.add_argument("--max-per-trade-risk-percentage", type=float, default=None)
    args.add_argument("--sl-shift", type=float, default=0.0)
    args.add_argument("--tp-shift", type=float, default=0.0)
    args.add_argument("--sl-buffer", type=float, default=0.0)
    args.add_argument("--tp-buffer", type=float, default=0.0)
    args.add_argument("--min-max-window-in-hours", type=int, default=4)
    args.add_argument("--use-htf-data", default=False, help="Use higher time frame data for the open strategy")
    
    args = args.parse_args()
        
    kwargs = {k:v for k, v in vars(args).items() if v is not None}
    
    logger.info(f"starting trading engine with kwargs: {kwargs}")
    
    profit, meta = objective(logger, kwargs)
    
    logger.info(f"profit: {profit}, meta: {meta}")
    