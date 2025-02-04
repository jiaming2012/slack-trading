from loguru import logger
from trading_engine import objective
from skopt import gp_minimize
from skopt.space import Real, Integer
from skopt.utils import use_named_args
from pprint import pprint
import os

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
            Real(-10.0, 10.0, name='sl_shift'),
            Real(-10.0, 10.0, name='tp_shift'),
            Real(0, 5, name='sl_buffer'),
            Real(0, 5, name='tp_buffer'),
            Integer(5, 24, name='min_max_window_in_hours')
        ]

        self.aggregate_meta = {}
        self.counter = 0
        self.n_calls = n_calls

    def fn(self, sl_shift, tp_shift, sl_buffer, tp_buffer, min_max_window_in_hours):
        global counter
        value, meta = objective(sl_shift, tp_shift, sl_buffer, tp_buffer, min_max_window_in_hours)
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