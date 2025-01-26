from src.cmd.stats.playground_metrics import collect_data
from rpc.playground_twirp import PlaygroundServiceClient
from rpc.playground_pb2 import CreatePolygonPlaygroundRequest, Repository
from src.cmd.stats.backtester_playground_client_grpc import BacktesterPlaygroundClient
from src.cmd.stats.playground_types import RepositorySource, OrderSide

twirp_host = "http://localhost:5051"
initial_balance = 25000
symbol = 'COIN'

repo = Repository(
    symbol="AAPL",
    timespan_multiplier=1,
    timespan_unit="minute",
    indicators=['supertrend'],
    history_in_days=10
)

request = CreatePolygonPlaygroundRequest(
    balance=initial_balance, 
    start_date="2021-01-04", 
    stop_date="2021-12-29",
    repositories=[repo],
    environment='simulator'
)

playground_client = BacktesterPlaygroundClient(request, RepositorySource.POLYGON, twirp_host)

print(f"performing integration test on playground id: {playground_client.id}")

i = 0
while not playground_client.is_backtest_complete():
    if i % 2 == 0:
        playground_client.place_order('AAPL', 10, OrderSide.BUY)
        playground_client.tick(100, raise_exception=True)
        playground_client.place_order('AAPL', 5, OrderSide.SELL)
        playground_client.tick(100, raise_exception=True)
        playground_client.place_order('AAPL', 5, OrderSide.SELL)
        playground_client.tick(100, raise_exception=True)
    else:
        playground_client.place_order('AAPL', 5, OrderSide.SELL_SHORT)
        playground_client.tick(100, raise_exception=True)
        playground_client.place_order('AAPL', 2, OrderSide.BUY_TO_COVER)
        playground_client.tick(100, raise_exception=True)
        playground_client.place_order('AAPL', 1, OrderSide.BUY_TO_COVER)
        playground_client.tick(100, raise_exception=True)
        playground_client.place_order('AAPL', 2, OrderSide.BUY_TO_COVER)
        playground_client.tick(100, raise_exception=True)
    
    account = playground_client.account
    realized_profit = account.balance - initial_balance
    
    data = collect_data(twirp_host, playground_client.id)
    expected_realized_profit = data['agg_data']['realized_profit']    
    
    # Show progress
    if i % 50 == 0:
        print(f"{i}: realized profit: {realized_profit}, expected realized profit: {expected_realized_profit} @ {playground_client.timestamp}")
    
    i += 1
    
    assert abs(realized_profit - expected_realized_profit) < 0.001, f"realized profit mismatch: {realized_profit} != {expected_realized_profit} @ {playground_client.timestamp}"

print("integration test passed")