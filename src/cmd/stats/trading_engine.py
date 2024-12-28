from simple_open_strategy import SimpleOpenStrategy, OpenSignal, OpenSignalName
from simple_close_strategy import SimpleCloseStrategy
from backtester_playground_client_grpc import BacktesterPlaygroundClient, OrderSide, RepositorySource, PlaygroundEnvironment
from typing import List, Tuple

# todo:
# refactor open_strategy to parameterize short and long periods
# use a logger

def get_sl_tp(signal: OpenSignal) -> Tuple[float, float]:
    sl = signal.min_price_prediction
    tp = signal.max_price_prediction
    return sl, tp
    
def build_tag(side: OrderSide, current_price: float, min_value:float, min_value_sd: float, max_value: float, max_value_sd) -> str:
    """ Builds a tag for the order based on the current price and the min and max values.
        min_value and max_value are the min and max values of the price prediction.
        min_value_sd and max_value_sd are the standard deviations of the min and max values.
        Parses the tag on the order in the format sl__{sl}__tp__{tp}, e.g. sl__100_50__tp__200_00
    """
    if not current_price:
        raise ValueError("current_price not found")
    
    min_value = min(min_value, current_price)
    max_value = max(max_value, current_price)
        
    if side == OrderSide.BUY:
        tp_target = max_value
        if tp_target <= current_price:
            raise ValueError(f"Invalid target price: tp_target of {tp_target} < current price of {current_price}")
        
        # sl_target = min_value - (0.5 * min_value_sd)
        sl_target = min_value
        
        tp = str(round(tp_target, 2)).replace('.', '_')
        sl = str(round(sl_target, 2)).replace('.', '_')
    elif side == OrderSide.SELL_SHORT:
        tp_target = min_value
        if tp_target >= current_price:
            raise ValueError(f"Invalid target price: tp_target of {tp_target} > current price of {current_price}")
        
        # sl_target = max_value + (0.5 * max_value_sd)
        sl_target = max_value
        
        tp = str(round(tp_target, 2)).replace('.', '_')
        sl = str(round(sl_target, 2)).replace('.', '_')
    elif side == OrderSide.SELL:
        return "close_all"
    elif side == OrderSide.BUY_TO_COVER:
        return "close_all"
    else:
        raise ValueError("Invalid side")
    
    return f"sl__{sl}__tp__{tp}"

if __name__ == "__main__":
    # meta parameters
    playground_tick_in_seconds = 300
    model_training_period_in_months = 12
    
    # input parameters
    balance = 100000
    symbol = 'COIN'
    start_date = '2024-10-10'
    end_date = '2024-11-10'
    repository_source = RepositorySource.POLYGON
    csv_path = None
    grpc_host = 'http://localhost:5051'
    env = PlaygroundEnvironment.SIMULATOR
    
    playground = BacktesterPlaygroundClient(balance, symbol, start_date, end_date, repository_source, env, csv_path, grpc_host=grpc_host)
    playground.tick(0)  # initialize the playground
    
    print(f"playground id: {playground.id}")
    
    open_strategy = SimpleOpenStrategy(playground, model_training_period_in_months)
    close_strategy = SimpleCloseStrategy(playground)
    
    while not open_strategy.is_complete():
        try:
            current_price = playground.get_current_candle(symbol, period=playground_tick_in_seconds).close
        except Exception as e:
            current_price = None
            print(f"Error getting current price: {e}")
            
        # check for close signals
        close_signals = close_strategy.tick(current_price)
        for s in close_signals:
            resp = playground.place_order(s.Symbol, s.Volume, s.Side, s.Reason)
            print(f"Placed close order: {resp}")

        # check for open signals
        signals = open_strategy.tick()
        position = None
        if len(signals) > 0:
            pos = playground.account.get_position(symbol)
            position = pos.quantity if pos else 0
                
        if len(signals) > 1:
            print(f"[ERROR] Multiple signals detected: {signals}")
            
        for s in signals:            
            if s.name == OpenSignalName.CROSS_ABOVE_20:
                if position < 0:
                    volume = abs(position)
                    side = OrderSide.BUY_TO_COVER
                    resp = playground.place_order(symbol, volume, side, tag)
                    print(f"Placed close order: {resp}")
                    playground.tick(0)

                side = OrderSide.BUY
            elif s.name == OpenSignalName.CROSS_BELOW_80:
                if position > 0:
                    volume = position
                    side = OrderSide.SELL
                    resp = playground.place_order(symbol, volume, side, tag)
                    print(f"Placed close order: {resp}")
                    playground.tick(0)
                    
                side = OrderSide.SELL_SHORT
            else:
                print(f"Unknown signal: {s.name}")
                continue
            
            try:
                volume = 10.0
                tag = build_tag(side, current_price, s.min_price_prediction, s.min_price_prediction_std_dev, s.max_price_prediction, s.max_price_prediction_std_dev)
            except ValueError as e:
                print(f"Error building tag: {e}. Skipping order ...")
                continue
            
            resp = playground.place_order(symbol, volume, side, tag)
            print(f"Placed open order: {resp}")
            
        playground.tick(playground_tick_in_seconds)
            
    print("Done")