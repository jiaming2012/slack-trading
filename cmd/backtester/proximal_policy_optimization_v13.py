import logging
import logging.handlers
import queue
import threading
import atexit
import gymnasium as gym
from gymnasium import spaces
import numpy as np
import pandas as pd
from stable_baselines3 import PPO
from stable_baselines3.common.vec_env import DummyVecEnv
from stable_baselines3.common.monitor import Monitor
from stable_baselines3.common.results_plotter import load_results, ts2xy, plot_results, X_TIMESTEPS
from lib import RenkoWS
from datetime import datetime
import pandas_ta as ta
import random
import argparse
import os
import torch
import math

print('cuda available: ', torch.cuda.is_available())  # Should return True if GPU is available

# from stable_baselines3.common.noise import NormalActionNoise
import matplotlib.pyplot as plt

from backtester_playground_client_grpc import BacktesterPlaygroundClient, OrderSide, RepositorySource

class TradingEnv(gym.Env):
    """
    Custom Environment for Renko Chart Trading using PPO and Sortino Ratio as reward.
    """
    metadata = {'render.modes': ['human']}
    
    def initialize(self):
        if self.terminal_equity is None:
            balance = self.initial_balance
        elif self.terminal_equity < 1000:
            balance = self.initial_balance
        else:
            balance = self.terminal_equity
        
        self.client = BacktesterPlaygroundClient(balance, self.symbol, self.start_date, self.end_date, self.repository_source, self.csv_path, grpc_host=self.grpc_host)
        
        if self.is_training:
            random_tick = self.pick_random_tick(self.start_date, self.end_date)
            self.client.tick(random_tick)
            tick_delta = self.client.flush_tick_delta_buffer()[0]
            self.logger.info(f'Random tick: {random_tick}, Start simulation at: {tick_delta.current_time}')
        
        # terminal balances
        self.max_drawdown_equity = balance * 0.8
        self.target_equity = balance * 1.3
        
        # temp
        self.current_observation = None
        
        self.previous_balance = balance
        self.current_step = 0
        self.returns = []
        self.negative_returns = []
        self.equity_history = []
        self.renko = None
        self.renko_brick_size = 3
        self.is_backtest_complete = False
        self.sl = 0
        self.tp = 0
        self.current_price = 0
        self.ma = 0
        self.timestamp = None
        self._internal_timestamp = None
        self.total_commission = 0
        self.rewards_history = []
        self.df = pd.DataFrame([])
        self.supertrend = None
        self.supertrend_direction = None
        self.supertrend_cls_price_diff = None
        self.average_equity = balance
        
        self.logger.info(f'Running simulation in playground {self.client.id}')
        
    def set_repository(self, repository_source, csv_path):
        self.repository_source = repository_source
        self.csv_path = csv_path
        
    def pick_random_tick(self, start_date: str, end_date: str) -> int:
        start = datetime.strptime(start_date, '%Y-%m-%d')
        end = datetime.strptime(end_date, '%Y-%m-%d')
        delta = end - start
        return random.randint(0, delta.total_seconds())
                
    def __init__(self, start_date, end_date, grpc_host, logger, initial_balance=10000, repository_source=RepositorySource.CSV, csv_path='training_data.csv', is_training=True):
        super(TradingEnv, self).__init__()
        
        # Parameters and variables
        self.symbol = 'TSLA'
        self.grpc_host = grpc_host
        self.start_date = start_date
        self.end_date = end_date
        self.initial_balance = initial_balance
        self.repository_source = repository_source
        self.csv_path = csv_path
        self.timestamp = None
        self._internal_timestamp = None
        self.rewards_history = []
        self.per_trade_commission = 0.025
        self.terminal_equity = None
        self.reset_count = 0
        self.is_training = is_training
        self.df = None
        self.logger = logger
        
        self.logger.setLevel(logging.DEBUG)
        
        self.action_space = spaces.Box(
            low=np.array([-1.0]),
            high=np.array([1.0]),
            dtype=np.float64
        )

        self.observation_space = spaces.Box(low=-np.inf, high=np.inf, shape=(48,), dtype=np.float64)
    
    def get_position_size(self, unit_quantity: float) -> float:
        if self.client.position > 0 and unit_quantity < 0:
            return self.client.position * unit_quantity
        elif self.client.position < 0 and unit_quantity > 0:
            return abs(self.client.position) * unit_quantity
        else:
            current_candle = self.client.current_candle
            current_price = current_candle.close if current_candle else 0
            if current_price == 0:
                return 0
            
            if unit_quantity > 0:
                max_free_margin_per_trade = 0.9
            else:
                max_free_margin_per_trade = 0.65
                
            if abs(unit_quantity) > 0.1:
                position = (self.client.account.free_margin * max_free_margin_per_trade * unit_quantity) / current_price
            else:
                position = 0
                
            return position
    
    def show_progress(self):
        if self.timestamp is None:
            return None
            
        if self._internal_timestamp is None:
            self._internal_timestamp = self.timestamp
            
        t1 = datetime.strptime(self._internal_timestamp, '%Y-%m-%dT%H:%M:%S%z')
        t2 = datetime.strptime(self.timestamp, '%Y-%m-%dT%H:%M:%S%z')
        
        days_elapsed = (t2 - t1).days
        if days_elapsed >= 1:
            self._internal_timestamp = self.timestamp
            self.render()
            
    def get_average_reward(self) -> float:
        result = np.mean(self.rewards_history)
        return 0 if np.isnan(result) else result
        
    def get_batch_size(self) -> int:        
        avg_reward = self.get_average_reward()
        if avg_reward <= 0:
            return 1500
        elif avg_reward < 10:
            return 3000
        else:
            return 5000
            
    def found_insufficient_free_margin(self, tick_delta: object) -> bool:
        invalid_orders = tick_delta.get('invalid_orders')
        
        if invalid_orders:
            for order in invalid_orders:
                if order.reject_reason and order.reject_reason.find('insufficient free margin') >= 0:
                    return True
                
        return False
    
    def found_liquidation(self, tick_delta: object) -> bool:
        events = tick_delta.get('events')
        
        if events:
            for event in events:
                if event.type == 'liquidation':
                    return True
                
        return False

    def get_reward(self, commission, position=0, is_close=False, include_pl=False):
        equity = self.client.account.equity
        free_margin = self.client.account.free_margin
  
        if free_margin > 0 :
            reward = (equity - self.average_equity) / free_margin
        else:
            reward = equity - self.average_equity
        
        return reward - commission
    
    def terminate_episode(self, terminated, truncated):
        reward = self.get_reward(0, include_pl=True)
        self.rewards_history.append(reward)
        self.render()
        
        self.logger.info('Backtest complete. Terminating episode ...')
        
        self.terminal_equity = self.client.account.equity
        
        result = self._get_observation(), reward, terminated, truncated, { 'equity': self.client.account.equity, 'timestamp': self.timestamp }
        return result
    
    def check_termainal_conditions(self):       
        # Ensure we are still within the data bounds
        if self.client.is_backtest_complete():            
            return self.terminate_episode(False, True)
        
        if self.client.account.equity <= 0:
            return self.terminate_episode(True, False)

        if self.is_training:
            if self.client.account.equity >= self.target_equity:
                return self.terminate_episode(True, False)

            if self.client.account.equity <= self.max_drawdown_equity:
                return self.terminate_episode(True, False)
 
        return None
    
    def step(self, action):
        if type(action) == np.ndarray:
            action = action[0]
            
        unit_quantity = action 
        position = self.get_position_size(unit_quantity)
        
        # Update the average equity
        equity = self.client.account.equity
        
        self.equity_history.append(equity)
        
        if len(self.equity_history) > 500:
            self.equity_history = self.equity_history[-500:]
            
        self.average_equity = np.mean(self.equity_history)
        
        # Check terminal conditions
        isFinished = self.check_termainal_conditions()
        if isFinished:
            return isFinished

        # Simulate trade, adjust balance, and calculate reward
        commission = 0
        seconds_elapsed = 60
        is_close = False
         
        if position > 0 and self.client.position >= 0:
            try:
                self.client.place_order(self.symbol, position, OrderSide.BUY)
            except Exception as e:
                pass
                # self.logger.error(f'Error placing order: {e}')
            
            self.client.tick(1)
            seconds_elapsed -= 1
            
            pl = self.client.account.pl
                
            # Check terminal conditions
            isFinished = self.check_termainal_conditions()
            if isFinished:
                return isFinished
            
            commission = self.per_trade_commission * position

        elif position < 0 and self.client.position <= 0:
            try:
                self.client.place_order(self.symbol, abs(position), OrderSide.SELL_SHORT)
            except Exception as e:
                pass
                # self.logger.error(f'Error placing order: {e}')
                
            self.client.tick(1)
            seconds_elapsed -= 1
            
            pl = self.client.account.pl
                
            # Check terminal conditions
            isFinished = self.check_termainal_conditions()
            if isFinished:
                return isFinished
                        
            commission = self.per_trade_commission * abs(position)
            
        elif position < 0 and self.client.position > 0:
            # close positive position
            current_position = self.client.position
            close_quantity = min(current_position, abs(position))
            is_close = True

            self.client.place_order(self.symbol, close_quantity, OrderSide.SELL)
            self.client.tick(1)
            seconds_elapsed -= 1
            
            pl = self.client.account.pl
            
            # Check terminal conditions
            isFinished = self.check_termainal_conditions()
            if isFinished:
                return isFinished
                            
            # open new short position
            remaining_position = position + current_position
            commission = 0
            
            if remaining_position < 0:
                try:
                    try:
                        self.client.place_order(self.symbol, abs(remaining_position), OrderSide.SELL_SHORT)
                    except Exception as e:
                        pass
                        # self.logger.error(f'Error placing order: {e}')
                        
                    self.client.tick(1)
                    seconds_elapsed -= 1
                    
                    pl = self.client.account.pl
                    
                    commission = self.per_trade_commission * abs(remaining_position)
                    
                    # Check terminal conditions
                    isFinished = self.check_termainal_conditions()
                    if isFinished:
                        return isFinished
                    
                except Exception as e:
                    raise(e)
            
        elif position > 0 and self.client.position < 0:
            # close negative position
            current_position = self.client.position
            close_quantity = min(abs(current_position), position)
            is_close = True
            
            self.client.place_order(self.symbol, close_quantity, OrderSide.BUY_TO_COVER)
            self.client.tick(1)
            seconds_elapsed -= 1
            
            pl = self.client.account.pl
            
            # Check terminal conditions
            isFinished = self.check_termainal_conditions()
            if isFinished:
                return isFinished
                
            commission = 0
            
            # open new long position
            remaining_position = position + current_position
            if remaining_position > 0:
                try:
                    try:
                        self.client.place_order(self.symbol, remaining_position, OrderSide.BUY)
                    except Exception as e:
                        pass
                        # self.logger.error(f'Error placing order: {e}')
                        
                    self.client.tick(1)
                    seconds_elapsed -= 1
                    
                    pl = self.client.account.pl
                    
                    commission = self.per_trade_commission * remaining_position
                    
                    # Check terminal conditions
                    isFinished = self.check_termainal_conditions()
                    if isFinished:
                        return isFinished
                                    
                except Exception as e:
                    raise(e)
                
                                    
        self.total_commission += commission
        
        # Move into the future by one step
        self.client.tick(seconds_elapsed)
        pl = self.client.account.pl
                
        if self.client.current_candle:
            self.timestamp = self.client.current_candle.datetime
            cls_price = self.client.current_candle.close
            timestampMs = datetime.strptime(self.client.current_candle.datetime, '%Y-%m-%dT%H:%M:%S%z').timestamp() * 1000
            
            if self.renko is None:
                self.renko = RenkoWS(timestampMs, cls_price, self.renko_brick_size, external_mode='normal')
            else:
                self.renko.add_prices(timestampMs, cls_price)
                
            # Update super trend
            new_row = {'timestamp': self.timestamp, 'high': self.client.current_candle.high, 'low': self.client.current_candle.low, 'close': self.client.current_candle.close}
            self.df = self.df.append(new_row, ignore_index=True)
            
            if len(self.df) > 50:
                if len(self.df) > 150:
                    self.df = self.df.tail(150)
                
                supertrend = ta.supertrend(self.df['high'], self.df['low'], self.df['close'], length=50, multiplier=3)

                self.supertrend = supertrend['SUPERT_50_3.0'].iloc[-1]
                self.supertrend_direction = supertrend['SUPERTd_50_3.0'].iloc[-1]
                self.supertrend_cls_price_diff = self.supertrend - cls_price
                
                    
        # Periodically show progress
        self.show_progress()
            
        # Update the account state
        reward = self.get_reward(commission, position=position, is_close=is_close, include_pl=False)
        self.rewards_history.append(reward)

        # Update the step and returns
        self.current_step += 1
        
        # self.returns.append(balance - self.initial_balance)
        observation = self._get_observation()
        
        # Include the balance in the info dictionary
        info = { 'equity': self.client.account.equity, 'timestamp': self.timestamp }
        
        result = observation, reward, False, False, info

        # Return the required 5 values for Gymnasium
        return result

    def get_observation(self):
        return self._get_observation()
    
    def _get_observation(self):
        # Get the last 40 prices, padded if necessary
        obs = np.zeros(40, dtype=np.float64)
        
        df = None
        if self.renko:
            df = self.renko.renko_animate()
            
        equity = self.client.account.equity
        avg_equity = self.average_equity
        free_margin = self.client.account.free_margin
        current_price = self.client.current_candle.close if self.client.current_candle else 0
        # liquidation_buffer = self.get_liquidation_buffer()
        
        supertrend = self.supertrend if self.supertrend else 0
        supertrend_direction = self.supertrend_direction if self.supertrend_direction else 0
        supertrend_cls_price_diff = self.supertrend_cls_price_diff if self.supertrend_cls_price_diff else 0

        if df is None or len(df) == 0:
            self.current_observation = np.append(obs, [supertrend, supertrend_direction, supertrend_cls_price_diff, current_price, equity, avg_equity, free_margin, self.client.position]).astype(np.float64)
            return self.current_observation
        
        # Take the last 20 prices
        df = df.tail(40)
        
        # from sklearn.preprocessing import MinMaxScaler
        # scaler = MinMaxScaler()
        # df['open'] = scaler.fit_transform(df[['open']])
                
        j = 0
        for i in range(len(df)):
            obs[j] = round(df.iloc[i].close - df.iloc[i].open)
            # obs[j+1] = df.iloc[i]['high']
            # obs[j+2] = df.iloc[i]['low']
            
            # j += 3
            j += 1
        
        self.current_observation = np.append(obs, [supertrend, supertrend_direction, supertrend_cls_price_diff, current_price, equity, avg_equity, free_margin, self.client.position]).astype(np.float64)
        return self.current_observation

    def reset(self, seed=None, options=None):
        super().reset(seed=seed)
        if seed is not None:
            np.random.seed(seed)
            
        self.reset_count += 1
                    
        self.initialize()
        
        # Return the initial observation and an empty info dictionary
        return self._get_observation(), {}
    
    def get_liquidation_buffer(self):
        equity = self.client.account.equity
        maintenance_margin = self.client.account.get_maintenance_margin(self.symbol)
        return equity - maintenance_margin

    def render(self, mode='human', close=False):
        equity = self.client.account.equity
        free_margin = self.client.account.free_margin
        liquidation_buffer = self.get_liquidation_buffer()
        pl = self.client.account.pl
        position = self.client.position
        avg_reward = np.mean(self.rewards_history) if len(self.rewards_history) > 0 else 0
        self.logger.info(f"Step: {self.current_step}, Tstamp: {self.timestamp}, Balance: {self.client.account.balance:.2f}, Equity: {equity:.2f}, Avg Equity: {self.average_equity:.2f}, Free Margin: {free_margin:.2f}, Liquidation Buffer: {liquidation_buffer:.2f}, PL: {pl:.2f}, Position: {position}, Total Commission: {self.total_commission:.2f}, Avg Reward: {avg_reward}") 

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--model', type=str, help='The name of the model to load', required=False)
    parser.add_argument('--timesteps', type=int, help='The number of timesteps to train the model', default=10)
    parser.add_argument('--host', type=str, help='The grpc host of the backtester playground', default='localhost:50051')
    
    args = parser.parse_args()

    projectsDir = os.getenv('PROJECTS_DIR')
    if projectsDir is None:
        raise ValueError('PROJECTS_DIR environment variable is not set')
    
    # Create a formatter that writes the log messages in a specific format
    start_time = datetime.now()
    
    # Create a queue for log messages
    log_queue = queue.Queue()

    # Create a handler that writes log messages to the queue
    queue_handler = logging.handlers.QueueHandler(log_queue)

    # Create a logger and add the queue handler to it
    logger = logging.getLogger(__name__)
    logger.setLevel(logging.DEBUG)
    logger.addHandler(queue_handler)

    # Create a handler that writes log messages to a file
    # Create log directory
    log_dir = os.path.join(projectsDir, 'slack-trading', 'cmd', 'backtester', 'logs')
    os.makedirs(log_dir, exist_ok=True)
    
    log_file = os.path.join(log_dir, f'{start_time.strftime("%Y-%m-%d-%H-%M-%S")}.log')
    file_handler = logging.FileHandler(log_file)
    file_handler.setLevel(logging.DEBUG)
    
    # Create a formatter that writes the log messages in a specific format
    formatter = logging.Formatter('%(asctime)s - %(levelname)s - %(message)s')
    file_handler.setFormatter(formatter)

    # Create a listener that reads log messages from the queue and writes them to the file
    queue_listener = logging.handlers.QueueListener(log_queue, file_handler)

    # Start the listener
    queue_listener.start()
    
    # Register a function to stop the listener at exit
    atexit.register(queue_listener.stop)

    # Initialize the environment
    start_date = '2024-03-18'
    end_date = '2024-09-13'
    env = TradingEnv(start_date, end_date, args.host, logger, initial_balance=10000, repository_source=RepositorySource.POLYGON)
    
    # Wrap the environment with Monitor
    env = Monitor(env, log_dir)

    # Wrap the environment with DummyVecEnv for compatibility with Stable-Baselines3
    vec_env = DummyVecEnv([lambda: env])

    if args.model is not None:
        # Load the PPO model
        loadModelDir = os.path.join(projectsDir, 'slack-trading', 'cmd', 'backtester', 'models')
        old_model = PPO.load(os.path.join(loadModelDir, args.model))
        # model.learning_rate = 0.00005
        old_model.set_env(vec_env)  # Assign the environment again (necessary for further training)

        model = PPO('MlpPolicy', vec_env, verbose=1, policy_kwargs={'net_arch': [64, 64, 64]}, ent_coef=0.01, learning_rate=0.0002)

        # Load parameters from the old model
        model.set_parameters(old_model.get_parameters())

        logger.info(f'Loaded model: {args.model}')
    else:
        # Create and train the PPO model
        model = PPO('MlpPolicy', vec_env, verbose=1, policy_kwargs={'net_arch': [64, 64, 64]}, ent_coef=0.01, learning_rate=0.0002)


    # Hyper parameters
    total_reset_counts = args.timesteps
    iterations = 1

    while env.reset_count < total_reset_counts:    
        isDone = False
        batch_size = env.get_batch_size()
        
        # while not isDone:
        # Train the model with the new experience
        logger.info(f'Training model with batch size: {batch_size} ...')
        
        try:
            model.learn(total_timesteps=batch_size, reset_num_timesteps=False)
        except Exception as e:
            # Save the trained model with timestep
            saveModelDir = os.path.join(projectsDir, 'slack-trading', 'cmd', 'backtester', 'models')
            modelName = 'ppo_model_v13-' + start_time.strftime('%Y-%m-%d-%H-%M-%S') + f'_{iterations}-terminated'
            model.save(os.path.join(saveModelDir, modelName))
            logger.info(f'Saved terminated model: {os.path.join(saveModelDir, modelName)}.zip')
            
            raise(e)
        
        # Print the current timestep and balance
        logger.info(f'Training complete. Reset count: {env.reset_count} / {total_reset_counts}.')
            
        logger.info('*' * 50)
        
        if iterations % 10 == 0:
            # Save the trained model with timestep
            saveModelDir = os.path.join(projectsDir, 'slack-trading', 'cmd', 'backtester', 'models')
            modelName = 'ppo_model_v13-' + start_time.strftime('%Y-%m-%d-%H-%M-%S') + f'_{iterations}'
            model.save(os.path.join(saveModelDir, modelName))
            logger.info(f'Saved intermediate model: {os.path.join(saveModelDir, modelName)}.zip')
            
        iterations += 1

        
    # Save the trained model with timestamp
    saveModelDir = os.path.join(projectsDir, 'slack-trading', 'cmd', 'backtester', 'models')
    modelName = 'ppo_model_v13-' + datetime.now().strftime('%Y-%m-%d-%H-%M-%S')
    model.save(os.path.join(saveModelDir, modelName))
    logger.info(f'Saved model: {os.path.join(saveModelDir, modelName)}.zip')