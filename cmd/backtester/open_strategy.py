from backtester_playground_client_grpc import BacktesterPlaygroundClient, OrderSide, RepositorySource, PlaygroundNotFoundException

if __name__ == "__main__":
    balance = 10000
    symbol = 'AAPL'
    start_date = '2024-06-03'
    end_date = '2024-09-30'
    repository_source = RepositorySource.POLYGON
    csv_path = None
    grpc_host = 'http://localhost:5051'
    
    client = BacktesterPlaygroundClient(balance, symbol, start_date, end_date, repository_source, csv_path, grpc_host=grpc_host)
    
    print(client.id)