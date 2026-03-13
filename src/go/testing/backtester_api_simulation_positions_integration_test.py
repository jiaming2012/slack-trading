from src.cmd.stats.playground_metrics import collect_data
from rpc.playground_twirp import PlaygroundServiceClient
from rpc.playground_pb2 import CreatePolygonPlaygroundRequest, Repository
from src.cmd.stats.backtester_playground_client_grpc import BacktesterPlaygroundClient
from src.cmd.stats.playground_types import RepositorySource, OrderSide
from dataclasses import dataclass
import random

random.seed(0)

@dataclass
class OrderParameter:
    quantity: int
    side: OrderSide

def generate_next_order(current_position: float) -> OrderParameter:
    if current_position > 0:
        return OrderParameter(quantity=random.randint(1, current_position), side=OrderSide.SELL)
    elif current_position < 0:
        return OrderParameter(quantity=random.randint(1, abs(current_position)), side=OrderSide.BUY_TO_COVER)
    else:
        if random.random() > 0.5:
            return OrderParameter(quantity=random.randint(1, 10), side=OrderSide.BUY)
        else:
            return OrderParameter(quantity=random.randint(1, 10), side=OrderSide.SELL_SHORT)

twirp_host = "http://localhost:5051"
initial_balance = 25000
symbol = 'AAPL'

repo = Repository(
    symbol=symbol,
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

expected_position = 0
i = 0
while not playground_client.is_backtest_complete():
    order_params = generate_next_order(expected_position)
    playground_client.place_order(symbol, order_params.quantity, order_params.side)
    playground_client.tick(100, raise_exception=True)
    
    account = playground_client.account
    actual_positions = account.positions
    actual_position = actual_positions[symbol].quantity if symbol in actual_positions else 0
    
    data = collect_data(twirp_host, playground_client.id)
    expected_position = data['gross_data']['positions'][symbol].quantity 
    
    # Show progress
    if i % 50 == 0:
        print(f"{i}: actual position: {actual_position}, expected position: {expected_position} @ {playground_client.timestamp}")
    
    i += 1
    
    assert abs(actual_position - expected_position) < 0.001, f"actual position mismatch: {actual_position} != {expected_position} @ {playground_client.timestamp}"

print("integration test passed")