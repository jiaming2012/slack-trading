
class SimpleBaseStrategy:
    def __init__(self, playground):
        self.playground = playground
        self.timestamp = playground.timestamp
        
    def is_complete(self):
        return self.playground.is_backtest_complete()
        
    def tick(self):
        raise Exception("Not implemented")