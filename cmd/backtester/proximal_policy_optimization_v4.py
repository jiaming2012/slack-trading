import gymnasium as gym
from gymnasium import spaces
import numpy as np
import pandas as pd
from stable_baselines3 import PPO
from stable_baselines3.common.vec_env import DummyVecEnv
from stable_baselines3.common.monitor import Monitor
from stable_baselines3.common.results_plotter import load_results, ts2xy, plot_results, X_TIMESTEPS
from lib import RenkoWS
from datetime import datetime, timedelta
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
        self.client = BacktesterPlaygroundClient(self.initial_balance, self.symbol, '2021-01-04', '2021-03-31', self.repository_source, self.csv_path)# , host='http://149.28.239.60')
        random_tick = self.pick_random_tick('2021-01-04', '2021-03-31')
        tick_delta = self.client.tick(random_tick)
        print(f'Random tick: {random_tick}, Start simulation at: {tick_delta.get("current_time")}')
        
        self.previous_balance = self.initial_balance
        self.current_step = 0
        self.returns = []
        self.negative_returns = []
        self.renko = None
        self.renko_brick_size = 10
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
                
    def __init__(self, initial_balance=10000, repository_source=RepositorySource.CSV, csv_path='training_data.csv'):
        super(RenkoTradingEnv, self).__init__()
        
        # Parameters and variables
        self.symbol = 'AAPL'
        self.initial_balance = initial_balance
        self.repository_source = repository_source
        self.csv_path = csv_path
        self.timestamp = None
        self._internal_timestamp = None
        self.rewards_history = []
        self.per_trade_commission = 0.1

        self.action_space = spaces.Box(
            low=np.array([-300]),
            high=np.array([300]),
            dtype=np.float64
        )

        # Observation space: Last 60 Renko blocks + balance, position, pl, free_margin, total_commission, liquidation_buffer
        self.observation_space = spaces.Box(low=-np.inf, high=np.inf, shape=(66,), dtype=np.float64)
        
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
            return 300
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

    def get_reward(self, commission, tick_delta=None, include_pl=False):
        balance = self.client.account.balance
        pl = 0
        
        if include_pl:
            pl = self.client.account.pl
        elif self.client.account.pl < 0:
            pl_ratio = abs(self.client.account.pl / balance) if balance > 0 else 0
            if pl_ratio > 0.1:
                pl = self.client.account.pl * 0.1
        
        result = balance - self.previous_balance - commission + pl
        
        if tick_delta:
            if self.found_insufficient_free_margin(tick_delta):
                result -= 100
                self.render()
                print('Insufficient free margin detected: subtracting 100 from reward.')
            
            if self.found_liquidation(tick_delta):
                result -= self.initial_balance
                self.render()
                print(f'Liquidation detected: subtracting {self.initial_balance} from reward.')
        
        self.previous_balance = balance
        return result
    
    def step(self, action):
        pl = self.client.account.pl
        position = round(action[0])  # Discrete action as integer

        terminated = False
        truncated = False
        if self.client.account.balance + pl <= 0:
            reward = self.get_reward(0, include_pl=True)
            self.rewards_history.append(reward)
            terminated = True
            self.render()
            print('Balance is zero or negative. Terminating episode ...')
            return self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance }

        # Ensure we are still within the data bounds
        if self.client.is_backtest_complete():
            reward = self.get_reward(0, include_pl=True)
            self.rewards_history.append(reward)
            truncated = True  # Episode truncated (e.g., max steps reached)
            self.render()
            print('Backtest is complete. Terminating episode ...')
            return self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance }

        # Simulate trade, adjust balance, and calculate reward
        
        commission = 0
        seconds_elapsed = 60
         
        if position > 0 and self.client.position >= 0:
            self.client.place_order(self.symbol, position, OrderSide.BUY)
            self.client.tick(1)
            seconds_elapsed -= 1
            
            pl = self.client.account.pl
                
            if self.client.is_backtest_complete():
                reward = self.get_reward(0, include_pl=True)
                self.rewards_history.append(reward)
                truncated = True  # Episode truncated (e.g., max steps reached)
                self.render()
                print('Backtest is complete. Terminating episode ...')
                return self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance, 'pl': pl }
            
            commission = self.per_trade_commission * position

        elif position < 0 and self.client.position <= 0:
            self.client.place_order(self.symbol, abs(position), OrderSide.SELL_SHORT)
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
                return self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance, 'pl': pl, 'position': self.client.position, 'current_price': self.current_price, 'ma': self.ma }
                        
            commission = self.per_trade_commission * abs(position)
            
        elif position < 0 and self.client.position > 0:
            # close positive position
            current_position = self.client.position
            close_quantity = min(current_position, abs(position))

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
                return self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance }
                            
            # open new short position
            remaining_position = position + current_position
            commission = 0
            
            if remaining_position < 0:
                try:
                    self.client.place_order(self.symbol, abs(remaining_position), OrderSide.SELL_SHORT)
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
                        return self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance }
                    
                except Exception as e:
                    raise(e)
            
        elif position > 0 and self.client.position < 0:
            # close negative position
            current_position = self.client.position
            close_quantity = min(abs(current_position), position)
            
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
                return self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance }
                
            commission = 0
            
            # open new long position
            remaining_position = position + current_position
            if remaining_position > 0:
                try:
                    self.client.place_order(self.symbol, remaining_position, OrderSide.BUY)
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
                        return self._get_observation(), reward, terminated, truncated, {'balance': self.client.account.balance }
                                    
                except Exception as e:
                    raise(e)
                
                                    
        self.total_commission += commission
        
        # Move into the future by one step
        tick_delta = self.client.tick(seconds_elapsed)
        pl = self.client.account.pl
                
        if self.client.current_candle:
            self.timestamp = self.client.current_candle['datetime']
            cls_price = self.client.current_candle['close']
            timestampMs = datetime.strptime(self.client.current_candle['datetime'], '%Y-%m-%dT%H:%M:%S%z').timestamp() * 1000
            
            if self.renko is None:
                self.renko = RenkoWS(timestampMs, cls_price, self.renko_brick_size, external_mode='nongap')
            else:
                self.renko.add_prices(timestampMs, cls_price)
                
        # Periodically show progress
        self.show_progress()
            
        # Update the account state
        reward = self.get_reward(commission, tick_delta=tick_delta, include_pl=False)
        self.rewards_history.append(reward)

        # Update the step and returns
        self.current_step += 1
        
        # self.returns.append(balance - self.initial_balance)
        observation = self._get_observation()
        
        # Include the balance in the info dictionary
        info = {'balance': self.client.account.balance }

        # Return the required 5 values for Gymnasium
        return observation, reward, truncated, terminated, info

    def _get_observation(self):
        # Get the last 300 prices, padded if necessary
        obs = np.zeros(60, dtype=np.float64)
        
        df = None
        if self.renko:
            df = self.renko.renko_animate()
            
        pl = self.client.account.pl
        free_margin = self.client.account.free_margin
        liquidation_buffer = self.get_liquidation_buffer()

        if df is None or len(df) == 0:
            return np.append(obs, [self.client.account.balance, self.client.position, pl, free_margin, self.total_commission, liquidation_buffer]).astype(np.float64)
        
        # Take the last 20 prices
        df = df.tail(20)
                
        j = 0
        for i in range(len(df)):
            obs[j] = df.iloc[i]['open']
            obs[j+1] = df.iloc[i]['high']
            obs[j+2] = df.iloc[i]['low']
            
            j += 3
        
        return np.append(obs, [self.client.account.balance, self.client.position, pl, free_margin, self.total_commission, liquidation_buffer]).astype(np.float64)

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
        position = self.client.account.get_position(self.symbol)
        avg_reward = np.mean(self.rewards_history) if len(self.rewards_history) > 0 else 0
        print(f"Step: {self.current_step}, Tstamp: {self.timestamp}, Balance: {self.client.account.balance:.2f}, Equity: {equity:.2f}, Free Margin: {free_margin:.2f}, Liquidation Buffer: {liquidation_buffer:.2f}, PL: {pl:.2f}, Position: {position}, Total Commission: {self.total_commission:.2f}, Avg Reward: {avg_reward:.2f}") 


parser = argparse.ArgumentParser()
parser.add_argument('--model', type=str, help='The name of the model to load')
parser.add_argument('--timestamps', type=int, help='The number of timestamps to train the model', default=75)
args = parser.parse_args()

projectsDir = os.getenv('PROJECTS_DIR')
if projectsDir is None:
    raise ValueError('PROJECTS_DIR environment variable is not set')

# Create log directory
log_dir = "tmp/"
os.makedirs(log_dir, exist_ok=True)

# Initialize the environment
env = RenkoTradingEnv(repository_source=RepositorySource.POLYGON)

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
total_timesteps = args.timestamps
batch_size = 100

for timestep in range(total_timesteps):    
    print(f'Training model with batch size: {batch_size} ...')
    
    model.learn(total_timesteps=batch_size, reset_num_timesteps=False)
    
    # Print the current timestep and balance
    print(f'Training complete @ Timestep {timestep}')

    vec_env.env_method('render', indices=0)
    
    print('*' * 50)
    
    batch_size = env.get_batch_size()

    vec_env.reset()
    
    if timestep % 20 == 0:
        # Save the trained model with timestep
        saveModelDir = os.path.join(projectsDir, 'slack-trading', 'cmd', 'backtester', 'models')
        modelName = 'ppo_model_v3-' + datetime.now().strftime('%Y-%m-%d-%H-%M-%S') + f'_{timestep}-of-{total_timesteps}'
        model.save(os.path.join(saveModelDir, modelName))
        print(f'Saved intermediate model: {os.path.join(saveModelDir, modelName, ".zip")}')


# Check if log files contain data
log_files = os.listdir(log_dir)
print(f"Log files: {log_files}")

# # Plot the results
# if log_files:
#     plot_results([log_dir], total_timesteps, X_TIMESTEPS, "PPO Renko Trading")
# else:
#     print("No log files found. Skipping plot.")
    
# Save the trained model with timestamp
saveModelDir = os.path.join(projectsDir, 'slack-trading', 'cmd', 'backtester', 'models')
modelName = 'ppo_model_v3-' + datetime.now().strftime('%Y-%m-%d-%H-%M-%S')
model.save(os.path.join(saveModelDir, modelName))
print(f'Saved model: {os.path.join(saveModelDir, modelName, ".zip")}')
