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
    
def build_tag(side: OrderSide, min_value:float, max_value: float) -> str:
    # parse the tag on the order in the format sl__{sl}__tp__{tp}, e.g. sl__100_50__tp__200_00
    if side == OrderSide.BUY:
        tp = str(round(max_value, 2)).replace('.', '_')
        sl = str(round(min_value, 2)).replace('.', '_')
    elif side == OrderSide.SELL_SHORT:
        tp = str(round(min_value, 2)).replace('.', '_')
        sl = str(round(max_value, 2)).replace('.', '_')
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
    
    # input parameters
    balance = 100000
    symbol = 'AAPL'
    start_date = '2024-10-10'
    end_date = '2024-11-10'
    repository_source = RepositorySource.POLYGON
    csv_path = None
    grpc_host = 'http://localhost:5051'
    env = PlaygroundEnvironment.SIMULATOR
    
    playground = BacktesterPlaygroundClient(balance, symbol, start_date, end_date, repository_source, env, csv_path, grpc_host=grpc_host)
    playground.tick(0)  # initialize the playground
    
    print(f"playground id: {playground.id}")
    
    model_training_period_in_months = 12
    open_strategy = SimpleOpenStrategy(playground, model_training_period_in_months)
    close_strategy = SimpleCloseStrategy(playground)
    
    while not open_strategy.is_complete():
        # check for close signals
        close_signals = close_strategy.tick()
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
            volume = 10.0
            
            if s.name == OpenSignalName.CROSS_ABOVE_20:
                if position >= 0:
                    side = OrderSide.BUY
                else:
                    volume = abs(position)
                    side = OrderSide.BUY_TO_COVER
                
            elif s.name == OpenSignalName.CROSS_BELOW_80:
                if position <= 0:
                    side = OrderSide.SELL_SHORT
                else:
                    volume = position
                    side = OrderSide.SELL
            else:
                print(f"Unknown signal: {s.name}")
                continue
            
            tag = build_tag(side, s.min_price_prediction, s.max_price_prediction)
            resp = playground.place_order(symbol, volume, side, tag)
            print(f"Placed open order: {resp}")
            
        playground.tick(playground_tick_in_seconds)
            
    print("Done")