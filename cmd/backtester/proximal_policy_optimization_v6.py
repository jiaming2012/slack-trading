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
import random
import argparse
import os
import torch

print('cuda available: ', torch.cuda.is_available())  # Should return True if GPU is available

# from stable_baselines3.common.noise import NormalActionNoise
import matplotlib.pyplot as plt
from backtester_playground_client import BacktesterPlaygroundClient, OrderSide, RepositorySource

class RenkoTradingEnv(gym.Env):
    """
    Custom Environment for Renko Chart Trading using PPO and Sortino Ratio as reward.
    """
    metadata = {'render.modes': ['human']}
    
    def initialize(self):
        self.client = BacktesterPlaygroundClient(self.initial_balance, self.symbol, self.start_date, self.end_date, self.repository_source, self.csv_path)# , host='http://149.28.239.60')
        random_tick = self.pick_random_tick(self.start_date, self.end_date)
        self.client.tick(random_tick)
        tick_delta = self.client.flush_tick_delta_buffer()[0]
        print(f'Random tick: {random_tick}, Start simulation at: {tick_delta.get("current_time")}')
        
        # temp
        self.current_observation = None
        self.previous_observation = None 
        self.step_results = None
        self.step_results_complete = None
        
        self.previous_balance = self.initial_balance
        self.current_step = 0
        self.returns = []
        self.negative_returns = []
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
        
        print(f'Running simulation in playground {self.client.id}')
        
    def set_repository(self, repository_source, csv_path):
        self.repository_source = repository_source
        self.csv_path = csv_path
        
    def pick_random_tick(self, start_date: str, end_date: str) -> int:
        start = datetime.strptime(start_date, '%Y-%m-%d')
        end = datetime.strptime(end_date, '%Y-%m-%d')
        delta = end - start
        return random.randint(0, delta.total_seconds())
                
    def __init__(self, start_date, end_date, initial_balance=10000, repository_source=RepositorySource.CSV, csv_path='training_data.csv'):
        super(RenkoTradingEnv, self).__init__()
        
        # Parameters and variables
        self.symbol = 'TSLA'
        self.start_date = start_date
        self.end_date = end_date
        self.initial_balance = initial_balance
        self.repository_source = repository_source
        self.csv_path = csv_path
        self.timestamp = None
        self._internal_timestamp = None
        self.rewards_history = []
        self.per_trade_commission = 0.01
        
        self.action_space = spaces.Box(
            low=np.array([-1.0]),
            high=np.array([1.0]),
            dtype=np.float64
        )

        # Observation space: Last 20 Renko blocks + current price + balance, position, pl, free_margin, total_commission, liquidation_buffer
        self.observation_space = spaces.Box(low=-np.inf, high=np.inf, shape=(27,), dtype=np.float64)
    
    def get_position_size(self, unit_quantity: float) -> float:
        if self.client.position > 0 and unit_quantity < 0:
            return self.client.position * unit_quantity
        elif self.client.position < 0 and unit_quantity > 0:
            return abs(self.client.position) * unit_quantity
        else:
            current_candle = self.client.current_candle
            current_price = current_candle['close'] if current_candle else 0
            if current_price == 0:
                return 0
            
            max_free_margin_per_trade = 0.3
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
            return 500
        elif avg_reward < 10:
            return 1000
        elif avg_reward < 50:
            return 1500
        elif avg_reward < 100:
            return 2000
        else:
            return 3000
            
    def found_insufficient_free_margin(self, tick_delta: object) -> bool:
        invalid_orders = tick_delta.get('invalid_orders')
        
        if invalid_orders:
            for order in invalid_orders:
                if order['reject_reason'] and order['reject_reason'].find('insufficient free margin') >= 0:
                    return True
                
        return False
    
    def found_liquidation(self, tick_delta: object) -> bool:
        events = tick_delta.get('events')
        
        if events:
            for event in events:
                if event['type'] == 'liquidation':
                    return True
                
        return False

    def get_reward(self, commission, position=0, is_close=False, include_pl=False):
        balance = self.client.account.balance
        pl = 0
        
        if include_pl:
            pl = self.client.account.pl
        elif self.client.account.pl < 0:
            pl_ratio = abs(self.client.account.pl / balance) if balance > 0 else 0
            if pl_ratio > 0.1:
                pl = self.client.account.pl * 0.1
        
        result = balance - self.previous_balance - commission + pl
        
        # add preview penalty
        if position != 0 and self.client.current_candle and is_close:
            r = self.client.preview_tick(5 * 60)
            if not r['is_backtest_complete']:
                if r['new_candles'] and len(r['new_candles']) > 0:
                    future_candle = None
                    for candle in r['new_candles']:
                        if candle['symbol'] == self.symbol:
                            future_candle = candle['candle']
                            break
                        
                    if future_candle:
                        future_price = future_candle['close']
                        current_price = self.client.current_candle['close']
                        penalty = (future_price - current_price) * position * -1
                        result -= penalty
        
        deltas = self.client.flush_tick_delta_buffer()
        for tick_delta in deltas:
            # if self.found_insufficient_free_margin(tick_delta):
            #     print('Insufficient free margin detected.')
            
            if self.found_liquidation(tick_delta):
                print(f'Liquidation detected')
        
        self.previous_balance = balance
        return result
    
    def step(self, action):
        pl = self.client.account.pl
        if type(action) == np.ndarray:
            action = action[0]
            
        unit_quantity = round(action)  # Discrete action as integer
        position = self.get_position_size(unit_quantity)

        terminated = False
        truncated = False
        
        if self.client.account.balance + pl <= 0:
            reward = self.get_reward(0, include_pl=True)
            self.rewards_history.append(reward)
            terminated = True
            self.render()
            print('Balance is zero or negative. Terminating episode ...')
            self.step_results = self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance }
            return self.step_results

        # Ensure we are still within the data bounds
        if self.client.is_backtest_complete():
            reward = self.get_reward(0, include_pl=True)
            self.rewards_history.append(reward)
            truncated = True  # Episode truncated (e.g., max steps reached)
            self.render()
            print('Backtest is complete. Terminating episode ...')
            self.step_results = self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance }
            
            return self.step_results

        # Simulate trade, adjust balance, and calculate reward
        
        commission = 0
        seconds_elapsed = 60
        is_close = False
         
        if position > 0 and self.client.position >= 0:
            try:
                self.client.place_order(self.symbol, position, OrderSide.BUY)
            except Exception as e:
                # print(f'Error placing order: {e}') 
                pass   
            
            self.client.tick(1)
            seconds_elapsed -= 1
            
            pl = self.client.account.pl
                
            if self.client.is_backtest_complete():
                reward = self.get_reward(0, include_pl=True)
                self.rewards_history.append(reward)
                truncated = True  # Episode truncated (e.g., max steps reached)
                self.render()
                print('Backtest is complete. Terminating episode ...')
                
                self.step_results = self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance, 'pl': pl }
                return self.step_results
            
            commission = self.per_trade_commission * position

        elif position < 0 and self.client.position <= 0:
            try:
                self.client.place_order(self.symbol, abs(position), OrderSide.SELL_SHORT)
            except Exception as e:
                # print(f'Error placing order: {e}') 
                pass
                
            self.client.tick(1)
            seconds_elapsed -= 1
            
            pl = self.client.account.pl
                
            if self.client.is_backtest_complete():
                pl = self.client.account.pl
                reward = self.get_reward(0, include_pl=True)
                self.rewards_history.append(reward)
                truncated = True  # Episode truncated (e.g., max steps reached)
                self.render()
                print('Backtest is complete. Terminating episode ...')
                self.step_results = self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance, 'pl': pl, 'position': self.client.position, 'current_price': self.current_price, 'ma': self.ma }
                
                return self.step_results
                        
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
            
            if self.client.is_backtest_complete():
                reward = self.get_reward(0, include_pl=True)
                self.rewards_history.append(reward)
                truncated = True  # Episode truncated (e.g., max steps reached)
                self.render()
                print('Backtest is complete. Terminating episode ...')
                
                self.step_results = self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance }
                return self.step_results
                            
            # open new short position
            remaining_position = position + current_position
            commission = 0
            
            if remaining_position < 0:
                try:
                    try:
                        self.client.place_order(self.symbol, abs(remaining_position), OrderSide.SELL_SHORT)
                    except Exception as e:
                        # print(f'Error placing order: {e}') 
                        pass   
                        
                    self.client.tick(1)
                    seconds_elapsed -= 1
                    
                    pl = self.client.account.pl
                    
                    commission = self.per_trade_commission * abs(remaining_position)
                    
                    if self.client.is_backtest_complete():
                        reward = self.get_reward(0, include_pl=True)
                        self.rewards_history.append(reward)
                        truncated = True  # Episode truncated (e.g., max steps reached)
                        self.render()
                        print('Backtest is complete. Terminating episode ...')
                        
                        self.step_results = self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance }
                        return self.step_results
                    
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
            
            if self.client.is_backtest_complete():
                pl = self.client.account.pl
                reward = self.get_reward(0, include_pl=True)
                self.rewards_history.append(reward)
                truncated = True  # Episode truncated (e.g., max steps reached)
                self.render()
                print('Backtest is complete. Terminating episode ...')
                
                self.step_results = self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance }
                return self.step_results
                
            commission = 0
            
            # open new long position
            remaining_position = position + current_position
            if remaining_position > 0:
                try:
                    try:
                        self.client.place_order(self.symbol, remaining_position, OrderSide.BUY)
                    except Exception as e:
                        # print(f'Error placing order: {e}')
                        pass
                        
                    self.client.tick(1)
                    seconds_elapsed -= 1
                    
                    pl = self.client.account.pl
                    
                    commission = self.per_trade_commission * remaining_position
                    
                    if self.client.is_backtest_complete():
                        reward = self.get_reward(0, include_pl=True)
                        self.rewards_history.append(reward)
                        truncated = True  # Episode truncated (e.g., max steps reached)
                        self.render()
                        print('Backtest is complete. Terminating episode ...')
                        self.step_results = self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance }
                        return self.step_results
                                    
                except Exception as e:
                    raise(e)
                
                                    
        self.total_commission += commission
        
        # Move into the future by one step
        self.client.tick(seconds_elapsed)
        pl = self.client.account.pl
                
        if self.client.current_candle:
            self.timestamp = self.client.current_candle['datetime']
            cls_price = self.client.current_candle['close']
            timestampMs = datetime.strptime(self.client.current_candle['datetime'], '%Y-%m-%dT%H:%M:%S%z').timestamp() * 1000
            
            if self.renko is None:
                self.renko = RenkoWS(timestampMs, cls_price, self.renko_brick_size, external_mode='normal')
            else:
                self.renko.add_prices(timestampMs, cls_price)
                
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
        info = {'balance': self.client.account.balance }
        
        self.step_results_complete = observation, reward, terminated, truncated, info

        # Return the required 5 values for Gymnasium
        return self.step_results_complete

    def get_observation(self):
        return self._get_observation()
    
    def _get_observation(self):
        # Get the last 300 prices, padded if necessary
        obs = np.zeros(20, dtype=np.float64)
        
        # temp
        self.previous_observation = self.current_observation
        
        df = None
        if self.renko:
            df = self.renko.renko_animate()
            
        balance = self.client.account.balance
        pl = self.client.account.pl
        current_price = self.client.current_candle['close'] if self.client.current_candle else 0
        free_margin_over_equity = self.client.get_free_margin_over_equity()
        liquidation_buffer = self.get_liquidation_buffer()

        if df is None or len(df) == 0:
            self.current_observation = np.append(obs, [current_price, balance, self.client.position, pl, free_margin_over_equity, self.total_commission, liquidation_buffer]).astype(np.float64)
            return self.current_observation
        
        # Take the last 20 prices
        df = df.tail(20)
                
        j = 0
        for i in range(len(df)):
            obs[j] = df.iloc[i]['open']
            # obs[j+1] = df.iloc[i]['high']
            # obs[j+2] = df.iloc[i]['low']
            
            # j += 3
            j += 1
        
        self.current_observation = np.append(obs, [current_price, balance, self.client.position, pl, free_margin_over_equity, self.total_commission, liquidation_buffer]).astype(np.float64)
        return self.current_observation

    def reset(self, seed=None, options=None):
        super().reset(seed=seed)
        if seed is not None:
            np.random.seed(seed)
                    
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
        print(f"Step: {self.current_step}, Tstamp: {self.timestamp}, Balance: {self.client.account.balance:.2f}, Equity: {equity:.2f}, Free Margin: {free_margin:.2f}, Liquidation Buffer: {liquidation_buffer:.2f}, PL: {pl:.2f}, Position: {position}, Total Commission: {self.total_commission:.2f}, Avg Reward: {avg_reward:.2f}") 

parser = argparse.ArgumentParser()
parser.add_argument('--model', type=str, help='The name of the model to load')
parser.add_argument('--timesteps', type=int, help='The number of timesteps to train the model', default=100)
args = parser.parse_args()

projectsDir = os.getenv('PROJECTS_DIR')
if projectsDir is None:
    raise ValueError('PROJECTS_DIR environment variable is not set')

start_time = datetime.now()

# Create log directory
log_dir = "tmp/"
os.makedirs(log_dir, exist_ok=True)

# Initialize the environment
start_date = '2024-01-03'
end_date = '2024-05-31'
env = RenkoTradingEnv(start_date, end_date, initial_balance=10000, repository_source=RepositorySource.POLYGON)

# Wrap the environment with Monitor
env = Monitor(env, log_dir)

# Wrap the environment with DummyVecEnv for compatibility with Stable-Baselines3
vec_env = DummyVecEnv([lambda: env])

if args.model is not None:
    # Load the PPO model
    loadModelDir = os.path.join(projectsDir, 'slack-trading', 'cmd', 'backtester', 'models')
    model = PPO.load(os.path.join(loadModelDir, args.model))
    model.set_env(vec_env)  # Assign the environment again (necessary for further training)
    print(f'Loaded model: {args.model}')
else:
    # Create and train the PPO model
    model = PPO('MlpPolicy', vec_env, verbose=1, policy_kwargs={'net_arch': [128, 128]}, ent_coef=0.5, learning_rate=0.001)


# Hyper parameters
total_timesteps = args.timesteps
batch_size = 500

for timestep in range(1, total_timesteps):    
    isDone = False
    batch_size = 1000
    
    # while not isDone:
    # Train the model with the new experience
    print(f'Training model after one week with batch size: {batch_size} ...')
    
    model.learn(total_timesteps=batch_size, reset_num_timesteps=False)
    
    # Print the current timestep and balance
    print(f'Training complete. Timestep: {timestep} / {total_timesteps}.')
        
    print('*' * 50)
    
    if timestep % 10 == 0:
        # Save the trained model with timestep
        saveModelDir = os.path.join(projectsDir, 'slack-trading', 'cmd', 'backtester', 'models')
        modelName = 'ppo_model_v6-' + start_time.strftime('%Y-%m-%d-%H-%M-%S') + f'-{timestep}-of-{total_timesteps}'
        model.save(os.path.join(saveModelDir, modelName))
        print(f'Saved intermediate model: {os.path.join(saveModelDir, modelName)}.zip')
        
        # Reset the environment
        print('Resetting environment ...')
        vec_env.reset()


# Check if log files contain data
log_files = os.listdir(log_dir)
print(f"Log files: {log_files}")
    
# Save the trained model with timestamp
saveModelDir = os.path.join(projectsDir, 'slack-trading', 'cmd', 'backtester', 'models')
modelName = 'ppo_model_v6-' + datetime.now().strftime('%Y-%m-%d-%H-%M-%S')
model.save(os.path.join(saveModelDir, modelName))
print(f'Saved model: {os.path.join(saveModelDir, modelName)}.zip')