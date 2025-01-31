from typing import List
from rpc.playground_pb2 import TickDelta
from trading_engine_types import OpenSignal

class BaseOpenStrategy:
    def __init__(self, playground):
        self.playground = playground
        self.timestamp = playground.timestamp
        
    def is_complete(self):
        return self.playground.is_backtest_complete()
        
    def tick(self, tick_delta: List[TickDelta]) -> List[OpenSignal]:
        raise Exception("Not implemented")