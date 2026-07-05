from typing import List, Dict
from enum import Enum
import numpy as np

# Define an enumeration for signals
class Signal(Enum):
    BUY = 'BUY'
    SELL = 'SELL'
    HOLD = 'HOLD'

class GridStrategy:
    def __init__(self, lower_bound: float, upper_bound: float, grid_size: int, investment_per_grid: float):
        """
        Initialize the grid strategy parameters.

        :param lower_bound: Lower price for the grid.
        :param upper_bound: Upper price for the grid.
        :param grid_size: Number of grid levels between lower and upper bounds.
        :param investment_per_grid: Amount to invest per grid level.
        """
        self.lower_bound = lower_bound
        self.upper_bound = upper_bound
        self.grid_size = grid_size
        self.investment_per_grid = investment_per_grid
        self.grid_levels = np.linspace(lower_bound, upper_bound, grid_size)
        self.active_positions: Dict[float, Signal] = {}

    def generate_signal(self, candle: Dict[str, float]) -> Signal:
        """
        Generate buy/sell signal based on the current candle data.

        :param candle: A dictionary containing 'open', 'high', 'low', 'close' price.
        :return: Signal Enum (BUY, SELL, or HOLD).
        """
        current_price = candle['close']
        signal = Signal.HOLD  # Default signal is to hold

        # Check if current price has crossed any grid level
        for level in self.grid_levels:
            if current_price <= level and level not in self.active_positions:
                # Buy signal: Price is below or at the grid level
                self.active_positions[level] = Signal.BUY
                signal = Signal.BUY
                print(f"Buy at grid level: {level}")
                break
            elif current_price >= level and self.active_positions.get(level) == Signal.BUY:
                # Sell signal: Price is above or at the grid level and was bought earlier
                del self.active_positions[level]
                signal = Signal.SELL
                print(f"Sell at grid level: {level}")
                break
        
        return signal

# Example of how you might stream data into the strategy
def stream_candles(strategy: GridStrategy, candle_data_stream: List[Dict[str, float]]) -> None:
    """
    Simulate candle data stream.

    :param strategy: Instance of GridStrategy.
    :param candle_data_stream: List of candle data dictionaries.
    """
    for candle in candle_data_stream:
        signal = strategy.generate_signal(candle)
        print(f"Signal: {signal.value}, Candle: {candle}")

# Example candle data
candle_data_stream: List[Dict[str, float]] = [
    {'open': 100, 'high': 105, 'low': 98, 'close': 102},
    {'open': 102, 'high': 108, 'low': 101, 'close': 104},
    {'open': 104, 'high': 110, 'low': 103, 'close': 107},
    {'open': 107, 'high': 111, 'low': 106, 'close': 109},
]

# Initialize the grid strategy
strategy = GridStrategy(lower_bound=100, upper_bound=110, grid_size=5, investment_per_grid=1000)

# Simulate candle data stream
stream_candles(strategy, candle_data_stream)
