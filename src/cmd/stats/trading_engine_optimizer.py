from loguru import logger
from trading_engine import objective
from skopt import gp_minimize
from skopt.space import Real, Integer
from skopt.utils import use_named_args
from datetime import datetime
import argparse
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
    def __init__(self, kwargs: dict):
        max_open_count_lbound = int(kwargs['max_open_count_lbound']) 
        max_open_count_ubound = int(kwargs['max_open_count_ubound']) 
        sl_buffer_ubound = kwargs.get('sl_buffer_ubound', None)
        tp_buffer_ubound = kwargs.get('tp_buffer_ubound', None)
        sl_shift_lbound = kwargs.get('sl_shift_lbound', None)
        sl_shift_ubound = kwargs.get('sl_shift_ubound', None)
        tp_shift_lbound = kwargs.get('tp_shift_lbound', None)
        tp_shift_ubound = kwargs.get('tp_shift_ubound', None)
        min_max_window_in_hours_lbound = kwargs.get('min_max_window_in_hours_lbound', None)
        min_max_window_in_hours_ubound = kwargs.get('min_max_window_in_hours_ubound', None)
        target_risk_to_reward_lbound = kwargs.get('target_risk_to_reward_lbound', None)
        target_risk_to_reward_ubound = kwargs.get('target_risk_to_reward_ubound', None)
        max_per_trade_risk_percentage_lbound = kwargs.get('max_per_trade_risk_percentage_lbound', None)
        max_per_trade_risk_percentage_ubound = kwargs.get('max_per_trade_risk_percentage_ubound', None)
        
        self.search_space = []
        
        if max_per_trade_risk_percentage_lbound is not None and max_per_trade_risk_percentage_ubound is not None:
            self.search_space.append(Real(float(max_per_trade_risk_percentage_lbound), float(max_per_trade_risk_percentage_ubound), name='max_per_trade_risk_percentage'))
        
        if target_risk_to_reward_lbound is not None and target_risk_to_reward_ubound is not None:
            self.search_space.append(Real(float(target_risk_to_reward_lbound), float(target_risk_to_reward_ubound), name='target_risk_to_reward'))
        
        if max_open_count_lbound is not None and max_open_count_ubound is not None:
            self.search_space.append(Integer(int(max_open_count_lbound), int(max_open_count_ubound), name='max_open_count'))
        
        if sl_buffer_ubound is not None:
            self.search_space.append(Real(0, int(sl_buffer_ubound), name='sl_buffer'))
            
        if tp_buffer_ubound is not None:
            self.search_space.append(Real(0, int(tp_buffer_ubound), name='tp_buffer'))
            
        if sl_shift_lbound is not None and sl_shift_ubound is not None:
            self.search_space.append(Real(int(sl_shift_lbound), int(sl_shift_ubound), name='sl_shift'))
            
        if tp_shift_lbound is not None and tp_shift_ubound is not None:
            self.search_space.append(Real(int(tp_shift_lbound), int(tp_shift_ubound), name='tp_shift'))

        if min_max_window_in_hours_lbound is not None and min_max_window_in_hours_ubound is not None:
            self.search_space.append(Integer(int(min_max_window_in_hours_lbound), int(min_max_window_in_hours_ubound), name='min_max_window_in_hours'))

        self.aggregate_meta = {}
        self.counter = 0
        self.n_calls = kwargs['n_calls']

    # TODO: Maybe this can be imported
    def fn(self, max_open_count: int, target_risk_to_reward: float, max_per_trade_risk_percentage: float, sl_buffer: float, tp_buffer: float):
        global counter
        kwargs = {
            'sl_buffer': sl_buffer,
            'tp_buffer': tp_buffer,
            'max_open_count': max_open_count,
            'target_risk_to_reward': target_risk_to_reward,
            'max_per_trade_risk_percentage': max_per_trade_risk_percentage,
        }
        
        logger.info(f"Running optimization with kwargs: {kwargs}")
        
        value, meta = objective(logger, kwargs=kwargs)
        
        meta_label = "_".join([f"{k}={v}" for k, v in kwargs.items()])
        meta['hyperparameters'] = kwargs
    
        self.aggregate_meta[meta_label] = meta
        self.counter += 1
    
        logger.info(f"Completed run: {self.counter} / {self.n_calls}, kwargs: {kwargs}, meta: {meta}")
    
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
    args = argparse.ArgumentParser()
    
    args.add_argument("--sl-buffer-lbound", type=float, default=0.0)
    args.add_argument("--sl-buffer-ubound", type=float, default=0.0)
    args.add_argument("--tp-buffer-lbound", type=float, default=0.0)
    args.add_argument("--tp-buffer-ubound", type=float, default=0.0)
    args.add_argument("--max-open-count-lbound", type=int, default=1)
    args.add_argument("--max-open-count-ubound", type=int, default=10)
    args.add_argument("--target-risk-to-reward-lbound", type=float, default=0.5)
    args.add_argument("--target-risk-to-reward-ubound", type=float, default=3.0)
    args.add_argument("--max-per-trade-risk-percentage-lbound", type=float, default=0.02)
    args.add_argument("--max-per-trade-risk-percentage-ubound", type=float, default=0.1)
    args.add_argument("--n-calls", type=int, default=15)
    
    args = args.parse_args()
    
    kwargs = {k:v for k, v in vars(args).items() if v is not None}
    
    logger.info(f"starting trading engine optimizer with kwargs: {kwargs}")
    
    # Run Bayesian optimization
    optimizer = TradingEngineOptimizer(kwargs)
    result = optimizer.optimize()
    
    # Pretty-print the aggregate_meta dictionary
    logger.info(f"Aggregate Meta: {optimizer.aggregate_meta}")
    
    avg_hyperparameters = optimizer.compute_average_hyperparameters(0.1)
    logger.info(f"Average Hyperparamters: {avg_hyperparameters}")
            
    # Print best parameters
    logger.info(f"Best results: {result.x}")