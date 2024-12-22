from backtester_playground_client_grpc import BacktesterPlaygroundClient, OrderSide, RepositorySource, PlaygroundNotFoundException
from generate_signals import new_supertrend_momentum_signal_factory
from dateutil.relativedelta import relativedelta


class BaseStrategy:
    def __init__(self, playground):
        self.playground = playground
        
        self.playground.tick(0)

        self.timestamp = playground.timestamp
        
    def is_complete(self):
        return self.playground.is_backtest_complete()
        
    def tick(self):
        raise Exception("Not implemented")
    
class SimpleOpenStrategy(BaseStrategy):
    def __init__(self, playground, tick_in_seconds=300):
        super().__init__(playground)
        
        self._tick_in_seconds = tick_in_seconds
        self.previous_month = None
        self.factory = None
        
    def is_new_month(self):
        current_month = self.playground.timestamp.month
        result = current_month != self.previous_month
        self.previous_month = current_month
        return result
    
    def get_previous_one_month_date_range(self):
        current_date = self.playground.timestamp
        first_day_of_current_month = current_date.replace(day=1)
        first_day_of_previous_month = first_day_of_current_month - relativedelta(months=1)
        
        start_date = first_day_of_previous_month
        end_date = first_day_of_current_month - relativedelta(days=1)
        
        return start_date, end_date
    
    def create_rolling_window(ltf_data, htf_data, start_date, end_date): 
        pass

    def tick(self):
        self.playground.tick(self._tick_in_seconds)
        
        tick_delta = self.playground.flush_tick_delta_buffer()
        new_candles = None
        for delta in tick_delta:
            if hasattr(delta, 'new_candles'):
                new_candles = delta.new_candles
                break
            
        if new_candles:
            pass
        
        if self.is_new_month():
            print(f"New month: {self.playground.timestamp}")
            start_date, end_date = self.get_previous_one_month_date_range()
            self.factory = new_supertrend_momentum_signal_factory(self.playground.symbol, start_date, end_date)
            
        if self.factory:
            pass
    

if __name__ == "__main__":
    balance = 10000
    symbol = 'AAPL'
    start_date = '2024-06-03'
    end_date = '2024-09-30'
    repository_source = RepositorySource.POLYGON
    csv_path = None
    grpc_host = 'http://localhost:5051'
    
    playground = BacktesterPlaygroundClient(balance, symbol, start_date, end_date, repository_source, csv_path, grpc_host=grpc_host)
    
    strategy = SimpleOpenStrategy(playground)
    
    while True:
        if not strategy.is_complete():
            strategy.tick()
        else:
            break
        
    print("Done")