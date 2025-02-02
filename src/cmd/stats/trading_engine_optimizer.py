from trading_engine import objective
from skopt import gp_minimize
from skopt.space import Real, Integer
from skopt.utils import use_named_args
from pprint import pprint

search_space = [
    Real(-10.0, 10.0, name='sl_shift'),
    Real(-10.0, 10.0, name='tp_shift'),
    Real(0, 5, name='sl_buffer'),
    Real(0, 5, name='tp_buffer'),
    Integer(5, 24, name='min_max_window_in_hours')
]

aggregate_meta = {}

@use_named_args(search_space)
def fn(sl_shift, tp_shift, sl_buffer, tp_buffer, min_max_window_in_hours):
    value, meta = objective(sl_shift, tp_shift, sl_buffer, tp_buffer, min_max_window_in_hours)
    meta_label = f"{sl_shift}_{tp_shift}_{sl_buffer}_{tp_buffer}_{min_max_window_in_hours}"
    aggregate_meta[meta_label] = meta
    
    return -value

if __name__ == '__main__':
    # Run Bayesian optimization
    result = gp_minimize(fn, search_space, n_calls=60, random_state=99)
    
    # Pretty-print the aggregate_meta dictionary
    print("Aggregate Meta:")
    pprint(aggregate_meta)
    
    # Print best parameters
    print(f"Best results: {result.x}")