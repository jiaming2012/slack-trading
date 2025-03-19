from loguru import logger
from trading_engine import objective
from skopt import gp_minimize
from skopt.space import Real, Integer
from skopt.utils import use_named_args
from pprint import pprint
from datetime import datetime
import os
import sys

logger.remove()

logger.add(sys.stdout, filter=lambda record: record["level"].name not in ["DEBUG", "WARNING"])

# Configure loguru to log to both console and file
logger = logger.bind(timestamp="", trading_operation="")
logger.add("trading_engine_{time}.log", format="timestamp={extra[timestamp]} trading_operation={extra[trading_operation]} {message}", rotation="1 day", retention="14 days", level="INFO")

env = os.getenv("PLAYGROUND_ENV")
if env == "live":
    level = "TRACE"
else:
    level = "INFO"

# Add a console sink
logger.add(
    sink=lambda msg: print(msg, end=""),  # Print to console
    format="{time:YYYY-MM-DD HH:mm:ss} | {level} | {message}",
    level=level
)

s = os.getenv("SYMBOL")

# Add a file sink
logger.add(
    f"logs/app-{s}-{datetime.now().strftime('%Y-%m-%d-%H-%M-%S')}.log",  # Log file name
    rotation="10 MB",  # Rotate when file size reaches 10MB
    retention="7 days",  # Keep logs for 7 days
    level=level,
    format="{time:YYYY-MM-DD HH:mm:ss} | {level} | {message}"
)

def compute_average_hyperparameters(sorted_meta: list) -> dict:
    avg_hyperparameters = {}
    for meta in sorted_meta:
        for key, value in meta[1]['hyperparameters'].items():
            if key not in avg_hyperparameters:
                avg_hyperparameters[key] = 0
            avg_hyperparameters[key] += value
    for key in avg_hyperparameters:
        avg_hyperparameters[key] /= len(sorted_meta)
    return avg_hyperparameters

def sort_meta_by_equity(data) -> list:
    sorted_meta = sorted(data.items(), key=lambda x: x[1]['equity'], reverse=True)
    return sorted_meta

class TradingEngineOptimizer:
    def __init__(self, n_calls: int):
        self.search_space = [
            Real(-15.0, 15.0, name='sl_shift'),
            Real(-15.0, 15.0, name='tp_shift'),
            Real(0, 5, name='sl_buffer'),
            Real(0, 5, name='tp_buffer'),
            Integer(10, 24, name='min_max_window_in_hours')
        ]

        self.aggregate_meta = {}
        self.counter = 0
        self.n_calls = n_calls

    def fn(self, sl_shift, tp_shift, sl_buffer, tp_buffer, min_max_window_in_hours):
        global counter
        value, meta = objective(logger, sl_shift, tp_shift, sl_buffer, tp_buffer, min_max_window_in_hours)
        meta_label = f"{sl_shift}_{tp_shift}_{sl_buffer}_{tp_buffer}_{min_max_window_in_hours}"
        meta['hyperparameters'] = {
            'sl_shift': sl_shift,
            'tp_shift': tp_shift,
            'sl_buffer': sl_buffer,
            'tp_buffer': tp_buffer,
            'min_max_window_in_hours': min_max_window_in_hours    
        }
    
        self.aggregate_meta[meta_label] = meta
        self.counter += 1
    
        logger.info(f"Completed run: {self.counter} / {self.n_calls}")
    
        return -value
    
    def compute_average_hyperparameters(self, top_percentile: float) -> dict:
        sorted_meta = sort_meta_by_equity(self.aggregate_meta)
        top_percentile_sorted_meta = sorted_meta[:int(len(sorted_meta) * top_percentile)]
        avg_hyperparameters = compute_average_hyperparameters(top_percentile_sorted_meta)
        return avg_hyperparameters
    
    def optimize(self):
        decorated_fn = use_named_args(self.search_space)(self.fn)
        result = gp_minimize(decorated_fn, self.search_space, n_calls=self.n_calls, random_state=99)
        return result

if __name__ == '__main__':
    n_calls = int(os.getenv('N_CALLS', None))
    
    # Run Bayesian optimization
    optimizer = TradingEngineOptimizer(n_calls)
    result = optimizer.optimize()
    
    # Pretty-print the aggregate_meta dictionary
    print("Aggregate Meta:")
    pprint(optimizer.aggregate_meta)
    
    print("Average Hyperparamters:")
    avg_hyperparameters = optimizer.compute_average_hyperparameters(0.1)
    pprint(avg_hyperparameters)
            
    # Print best parameters
    print(f"Best results: {result.x}")