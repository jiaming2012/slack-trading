from trading_engine import objective
from skopt import gp_minimize
from skopt.space import Real, Integer
from skopt.utils import use_named_args
from pprint import pprint
import os

search_space = [
    Real(-10.0, 10.0, name='sl_shift'),
    Real(-10.0, 10.0, name='tp_shift'),
    Real(0, 5, name='sl_buffer'),
    Real(0, 5, name='tp_buffer'),
    Integer(5, 24, name='min_max_window_in_hours')
]

aggregate_meta = {}
counter = 0
n_calls = os.getenv('N_CALLS', None)

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

def sort_meta_by_equity() -> list:
    sorted_meta = sorted(aggregate_meta.items(), key=lambda x: x[1]['equity'], reverse=True)
    return sorted_meta

@use_named_args(search_space)
def fn(sl_shift, tp_shift, sl_buffer, tp_buffer, min_max_window_in_hours):
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
    
    aggregate_meta[meta_label] = meta
    counter += 1
    
    print(f"Completed run: {counter} / {n_calls}")
    
    return -value

if __name__ == '__main__':
    # Run Bayesian optimization
    result = gp_minimize(fn, search_space, n_calls=n_calls, random_state=99)
    
    # Pretty-print the aggregate_meta dictionary
    print("Aggregate Meta:")
    pprint(aggregate_meta)
    
    print("Top 10% Sorted Meta:")
    sorted_meta = sort_meta_by_equity()
    top_percentile_sorted_meta = sorted_meta[:int(len(sorted_meta) * 0.1)]
    pprint(top_percentile_sorted_meta)
    
    print("Average Hyperparamters:")
    avg_hyperparameters = compute_average_hyperparameters(top_percentile_sorted_meta)
    pprint(avg_hyperparameters)
            
    # Print best parameters
    print(f"Best results: {result.x}")