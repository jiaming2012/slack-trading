from simple_open_strategy import SimpleOpenStrategy, OpenSignal, OpenSignalName
from simple_close_strategy import SimpleCloseStrategy
from playground_metrics import collect_data
from backtester_playground_client_grpc import BacktesterPlaygroundClient, OrderSide, RepositorySource, PlaygroundEnvironment, Repository, CreatePolygonPlaygroundRequest
from typing import List, Tuple
from datetime import datetime
import time
import os

# todo:
# refactor open_strategy to parameterize short and long periods
# use a logger

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
        

def calculate_new_trade_quantity(equity: float, free_margin: float, current_price: float, side: OrderSide, stop_loss: float, max_per_trade_risk_percentage: float) -> float:
    max_allowable_margin = free_margin * 0.99  # Change back to 25%
    max_per_trade_risk = equity * max_per_trade_risk_percentage
    
    sl_distance = abs(current_price - stop_loss)
    quantity = max_per_trade_risk / sl_distance

    required_margin = calculate_required_margin(current_price, quantity, side)
    if required_margin > max_allowable_margin:
        _quantity = max_allowable_margin / calculate_required_margin(current_price, 1, side)
        print(f"reducing quantity {quantity:.2f} -> {_quantity:.2f}: required_margin of {required_margin:.2f} > max_allowable_margin of {max_allowable_margin:.2f}")
        quantity = _quantity
    
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

def calculate_sl_tp(side: OrderSide, current_price: float, min_value:float, min_value_sd: float, max_value: float, max_value_sd: float, sl_shift: float, tp_shift: float) -> Tuple[float, float]:
    """ Builds a tag for the order based on the current price and the min and max values.
        min_value and max_value are the min and max values of the price prediction.
        min_value_sd and max_value_sd are the standard deviations of the min and max values.
    """
    if not current_price:
        raise ValueError("current_price not found")
    
    min_value = min(min_value, current_price)
    max_value = max(max_value, current_price)
        
    if side == OrderSide.BUY:
        tp_target = max_value + tp_shift
        if tp_target <= current_price:
            raise ValueError(f"Invalid target price: tp_target of {tp_target} <= current price of {current_price}")
        
        sl_target = min_value - sl_shift
        
    elif side == OrderSide.SELL_SHORT:
        tp_target = min_value - tp_shift
        if tp_target >= current_price:
            raise ValueError(f"Invalid target price: tp_target of {tp_target} >= current price of {current_price}")
        
        sl_target = max_value + sl_shift
        
    else:
        raise ValueError("Invalid side")
        
    return sl_target, tp_target
    
def objective(sl_shift = 0.0, tp_shift = 0.0) -> Tuple[float, dict]:
    # meta parameters
    model_training_period_in_months = 12
    
    # input parameters
    # Read environment variables
    balance = float(os.getenv("BALANCE"))
    symbol = os.getenv("SYMBOL")
    grpc_host = os.getenv("GRPC_HOST")
    playground_env = os.getenv("PLAYGROUND_ENV")
    live_account_type = os.getenv("LIVE_ACCOUNT_TYPE")

    # Check if the required environment variables are set
    if balance is None:
        raise ValueError("Environment variable BALANCE is not set")
    if symbol is None:
        raise ValueError("Environment variable SYMBOL is not set")
    if grpc_host is None:
        raise ValueError("Environment variable GRPC_HOST is not set")
    if playground_env is None:
        raise ValueError("Environment variable PLAYGROUND_ENV is not set")
    if playground_env.lower() == "live" and live_account_type is None:
        raise ValueError("Environment variable LIVE_ACCOUNT_TYPE is not set")
    
    if live_account_type is not None:
        print(f'info: starting {playground_env} playgound for {symbol} with account type {live_account_type}')
    else:
        print(f'info: starting {playground_env} playgound for {symbol}')
    
    if playground_env.lower() == "simulator":
        playground_tick_in_seconds = 300
        start_date = '2025-01-23'
        stop_date = '2025-01-30'
        repository_source = RepositorySource.POLYGON
        env = PlaygroundEnvironment.SIMULATOR
    
        print(f"initializing {env}: {symbol} playground from {start_date} to {stop_date} ...")
    elif playground_env.lower() == "live":
        playground_tick_in_seconds = 5
        start_date = None
        stop_date = None
        repository_source = None
        env = PlaygroundEnvironment.LIVE
        
        print(f"initializing {env}: {symbol} playground")
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
    
    playground = BacktesterPlaygroundClient(req, live_account_type, repository_source, grpc_host=grpc_host)
    
    playground.tick(0, raise_exception=False)  # initialize the playground
    
    print(f"playground id: {playground.id}")
    
    open_strategy = SimpleOpenStrategy(playground, model_training_period_in_months, sl_shift, tp_shift)
    close_strategy = SimpleCloseStrategy(playground)
    
    while not open_strategy.is_complete():
        try:
            current_price = playground.get_current_candle(symbol, period=ltf_period).close
        except Exception as e:
            current_price = None
            print(f"error: failed to get current price: {e}")
            
        # check for close signals
        close_signals = close_strategy.tick(current_price)
        for s in close_signals:
            resp = playground.place_order(s.Symbol, s.Volume, s.Side, current_price, s.Reason, raise_exception=False, with_tick=True)
            print(f"Placed close order: {resp}")

        # check for open signals
        tick_delta = playground.flush_new_state_buffer()
        for event in tick_delta:
            for trade in event.new_trades:
                print('-' * 40)
                print(f"{trade.symbol} placed: {trade.quantity} @ {trade.price} on {trade.create_date}")
                print('-' * 40)
        
        signals = open_strategy.tick(tick_delta)
        position = None
        if len(signals) > 0:
            pos = playground.account.get_position(symbol)
            position = pos.quantity if pos else 0
                
        if len(signals) > 1:
            print(f"[ERROR] Multiple signals detected: {signals}")
            
        for s in signals:            
            if s.name == OpenSignalName.CROSS_ABOVE_20:
                if position < 0:
                    qty = abs(position)
                    side = OrderSide.BUY_TO_COVER
                    resp = playground.place_order(symbol, qty, side, current_price, 'close-all', raise_exception=False, with_tick=True)
                    print(f"Placed close order: {resp}")

                side = OrderSide.BUY
            elif s.name == OpenSignalName.CROSS_BELOW_80:
                if position > 0:
                    qty = position
                    side = OrderSide.SELL
                    resp = playground.place_order(symbol, qty, side, current_price, 'close-all', raise_exception=False, with_tick=True)
                    print(f"Placed close order: {resp}")
                    
                side = OrderSide.SELL_SHORT
            else:
                print(f"Unknown signal: {s.name}")
                continue
            
            try:
                sl, tp = calculate_sl_tp(side, current_price, s.min_price_prediction, s.min_price_prediction_std_dev, s.max_price_prediction, s.max_price_prediction_std_dev, s.sl_shift, s.tp_shift)
                quantity = calculate_new_trade_quantity(playground.account.equity, playground.account.free_margin, current_price, side, s.min_price_prediction, 0.03)
                quantity = int(round(quantity - 0.5, 0))
                tag = build_tag(sl, tp, side)
            except ValueError as e:
                print(f"Error building tag: {e}. Skipping order ...")
                continue
            
            try:
                resp = playground.place_order(symbol, quantity, side, current_price, tag)
            except Exception as e:
                print(f"Error placing order: {e}")
                continue
            
            print(f"Placed open order: {resp}")
            
        playground.tick(playground_tick_in_seconds, raise_exception=False)
        
    profit = playground.account.equity - balance
    print(f"Playground: {playground.id} completed with profit of {profit:.2f} and (sl_shift, tp_shift) of ({sl_shift}, {tp_shift})")
    
    # fetch stats
    stats = collect_data(grpc_host, playground.id)
    
    meta = {
        'profit': profit,
        'sl_shift': sl_shift,
        'tp_shift': tp_shift,
        'equity': playground.account.equity,
        'stats': stats
    }
    
    playground.remove_from_server()
    
    return -profit, meta

if __name__ == "__main__":
    sl_shift = 0.0
    tp_shift = 0.0
    
    neg_profit, meta = objective(sl_shift, tp_shift)
    
    print(f"profit: {neg_profit}, meta: {meta}")
            
    