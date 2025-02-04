from abc import ABC, abstractmethod
from typing import List
from rpc.playground_pb2 import TickDelta
from trading_engine_types import OpenSignal

class BaseOpenStrategy(ABC):
    def __init__(self, playground):
        self.playground = playground
        self.timestamp = playground.timestamp
    
    def is_complete(self):
        return self.playground.is_backtest_complete()
    
    @abstractmethod        
    def get_sl_shift(self):
        pass
    
    @abstractmethod
    def get_tp_shift(self):
        pass
    
    @abstractmethod
    def get_sl_buffer(self):
        pass
    
    @abstractmethod
    def get_tp_buffer(self):
        pass
    
    @abstractmethod
    def tick(self, tick_delta: List[TickDelta]) -> List[OpenSignal]:
        pass