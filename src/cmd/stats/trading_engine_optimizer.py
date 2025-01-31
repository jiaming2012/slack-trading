from trading_engine import objective
from skopt import gp_minimize
from skopt.space import Real
from skopt.utils import use_named_args
from pprint import pprint

search_space = [
    Real(-10.0, 10.0, name='sl_shift'),
    Real(-10.0, 10.0, name='tp_shift')
]

aggregate_meta = {}

@use_named_args(search_space)
def fn(sl_shift, tp_shift):
    value, meta = objective(sl_shift, tp_shift)
    meta_label = f"{sl_shift}_{tp_shift}"
    aggregate_meta[meta_label] = meta
    
    return value

if __name__ == '__main__':
    # Run Bayesian optimization
    result = gp_minimize(fn, search_space, n_calls=30, random_state=42)
    
    # Pretty-print the aggregate_meta dictionary
    print("Aggregate Meta:")
    pprint(aggregate_meta)
    
    # Print best parameters
    best_sl_shift, best_tp_shift = result.x
    print(f"Best sl_shift: {best_sl_shift}, Best tp_shift: {best_tp_shift}")