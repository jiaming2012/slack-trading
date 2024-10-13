import gymnasium as gym
from gymnasium import spaces
import numpy as np
import pandas as pd
from stable_baselines3 import PPO
from stable_baselines3.common.vec_env import DummyVecEnv
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
        self.client = BacktesterPlaygroundClient(self.initial_balance, 'AAPL', '2021-01-04', '2021-01-29', self.repository_source, self.csv_path, host='http://149.28.239.60')
        self.previous_balance = self.initial_balance
        self.current_step = 0
        self.position = 0  # 1 for long, -1 for short, 0 for no position
        self.returns = []
        self.negative_returns = []
        self.recent_close_prices = np.array([])
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
        
    def __init__(self, initial_balance=10000, repository_source=RepositorySource.CSV, csv_path='training_data.csv'):
        super(RenkoTradingEnv, self).__init__()
        
        # Parameters and variables
        self.initial_balance = initial_balance
        self.repository_source = repository_source
        self.csv_path = csv_path
        self.position = None
        self.timestamp = None
        self._internal_timestamp = None
        self.rewards_history = []

        # Action space: Continuous (take_profit, stop_loss)
        # self.action_space = spaces.Box(low=np.array([50, 50, -3]), high=np.array([80, 80]), dtype=np.float32)
        self.action_space = spaces.Box(
            low=np.array([-3]),
            high=np.array([3]),
            dtype=np.float32
        )

        # Observation space: Last 10 Renko blocks + portfolio balance + pl + position
        self.observation_space = spaces.Box(low=-np.inf, high=np.inf, shape=(64,), dtype=np.float32)
        
    def print_current_state(self):
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

    def get_reward(self, commission, include_pl=False):
        balance = self.client.account.balance
        pl = 0
        
        if include_pl:
            pl = self.client.account.pl
        elif self.client.account.pl < 0:
            pl_ratio = abs(self.client.account.pl / balance)
            if pl_ratio > 0.1:
                pl = self.client.account.pl * 0.1
        
        result = balance - self.previous_balance - commission + pl
        self.previous_balance = balance
        return result
    
    def step(self, action):
        # Example custom logic to apply the action and calculate reward
        # renko_size = action[0]
        balance = self.client.account.balance
        pl = self.client.account.pl
        position = round(action[0])  # Discrete action as integer

        terminated = False
        truncated = False
        if balance + pl <= 0:
            reward = self.get_reward(0, include_pl=True)
            self.rewards_history.append(reward)
            terminated = True
            self.render()
            print('Balance is zero or negative. Terminating episode ...')
            return self._get_observation(), reward, terminated, truncated, {'balance': balance, 'pl': pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }

        # Ensure we are still within the data bounds
        if self.client.is_backtest_complete():
            reward = self.get_reward(0, include_pl=True)
            self.rewards_history.append(reward)
            truncated = True  # Episode truncated (e.g., max steps reached)
            self.render()
            print('Backtest is complete. Terminating episode ...')
            return self._get_observation(), reward, terminated, truncated, {'balance': balance, 'pl': pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }

        # Simulate trade, adjust balance, and calculate reward
        
        commission = 0
        seconds_elapsed = 60
         
        # if len(self.recent_close_prices) >= 5:
        if position > 0 and self.position >= 0:
            self.client.place_order('AAPL', position, OrderSide.BUY)
            
            cs = self.client.tick(1)
            balance = self.client.account.balance
            pl = self.client.account.pl
                
            if self.client.is_backtest_complete():
                reward = self.get_reward(0, include_pl=True)
                self.rewards_history.append(reward)
                truncated = True  # Episode truncated (e.g., max steps reached)
                self.render()
                print('Backtest is complete. Terminating episode ...')
                return self._get_observation(), reward, terminated, truncated, {'balance': balance, 'pl': pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }
            
            seconds_elapsed -= 1
                
            commission = 2 * position
            self.position += position
        elif position < 0 and self.position <= 0:
            self.client.place_order('AAPL', abs(position), OrderSide.SELL_SHORT)
            
            cs = self.client.tick(1)
            balance = self.client.account.balance
            pl = self.client.account.pl
                
            if self.client.is_backtest_complete():
                balance = self.client.account.balance
                pl = self.client.account.pl
                reward = self.get_reward(0, include_pl=True)
                self.rewards_history.append(reward)
                truncated = True  # Episode truncated (e.g., max steps reached)
                self.render()
                print('Backtest is complete. Terminating episode ...')
                return self._get_observation(), reward, terminated, truncated, {'balance': balance, 'pl': pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }
            
            seconds_elapsed -= 1
            
            commission = 2 * abs(position)
            self.position += position
        elif position < 0 and self.position > 0:
            # close positive position
            close_quantity = min(self.position, abs(position))
            self.client.place_order('AAPL', close_quantity, OrderSide.SELL)
            cs = self.client.tick(1)
            balance = self.client.account.balance
            pl = self.client.account.pl
            
            if self.client.is_backtest_complete():
                reward = self.get_reward(0, include_pl=True)
                self.rewards_history.append(reward)
                truncated = True  # Episode truncated (e.g., max steps reached)
                self.render()
                print('Backtest is complete. Terminating episode ...')
                return self._get_observation(), reward, terminated, truncated, {'balance': balance, 'pl': pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }
                
            seconds_elapsed -= 1
            
            # open new short position
            remaining_position = position + self.position
            commission = 0
            
            if remaining_position < 0:
                try:
                    self.client.place_order('AAPL', abs(remaining_position), OrderSide.SELL_SHORT)
                    
                    cs = self.client.tick(1)
                    balance = self.client.account.balance
                    pl = self.client.account.pl
                    commission = 2 * abs(remaining_position)
                    
                    if self.client.is_backtest_complete():
                        reward = self.get_reward(0, include_pl=True)
                        self.rewards_history.append(reward)
                        truncated = True  # Episode truncated (e.g., max steps reached)
                        self.render()
                        print('Backtest is complete. Terminating episode ...')
                        return self._get_observation(), reward, terminated, truncated, {'balance': balance, 'pl': pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }
                    
                    seconds_elapsed -= 1
                except Exception as e:
                    print('cs: ', cs)
                    raise(e)
            
            self.position = remaining_position
        elif position > 0 and self.position < 0:
            # close negative position
            close_quantity = min(abs(self.position), position)
            self.client.place_order('AAPL', close_quantity, OrderSide.BUY_TO_COVER)
            cs = self.client.tick(1)
            balance = self.client.account.balance
            pl = self.client.account.pl
            
            if self.client.is_backtest_complete():
                balance = self.client.account.balance
                pl = self.client.account.pl
                reward = self.get_reward(0, include_pl=True)
                self.rewards_history.append(reward)
                truncated = True  # Episode truncated (e.g., max steps reached)
                self.render()
                print('Backtest is complete. Terminating episode ...')
                return self._get_observation(), reward, terminated, truncated, {'balance': balance, 'pl': pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }
                
            seconds_elapsed -= 1
            commission = 0
            
            # open new long position
            remaining_position = position + self.position
            if remaining_position > 0:
                try:
                    self.client.place_order('AAPL', remaining_position, OrderSide.BUY)
                    cs = self.client.tick(1)
                    balance = self.client.account.balance
                    pl = self.client.account.pl
                    commission = 2 * remaining_position
                    
                    if self.client.is_backtest_complete():
                        reward = self.get_reward(0, include_pl=True)
                        self.rewards_history.append(reward)
                        truncated = True  # Episode truncated (e.g., max steps reached)
                        self.render()
                        print('Backtest is complete. Terminating episode ...')
                        return self._get_observation(), reward, terminated, truncated, {'balance': balance, 'pl': pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }
                    
                    seconds_elapsed -= 1
                
                except Exception as e:
                    print('cs: ', cs)
                    raise(e)
            
            self.position = remaining_position
    
                                    
        self.total_commission += commission
        
        # Move into the future by one step
        current_state = self.client.tick(seconds_elapsed)
        balance = self.client.account.balance
        pl = self.client.account.pl
        
        # reward = self.playground_client.fetch_reward_from_new_trades(current_state, stop_loss, take_profit, commission)
        
        if self.client.current_candle:
            self.timestamp = self.client.current_candle['datetime']
            cls_price = self.client.current_candle['close']
            self.recent_close_prices = np.append(self.recent_close_prices, cls_price)
            if len(self.recent_close_prices) > 300:
                self.recent_close_prices = self.recent_close_prices[1:]  # Remove the first element
            # convert timestamp to milliseconds
            timestampMs = datetime.strptime(self.client.current_candle['datetime'], '%Y-%m-%dT%H:%M:%S%z').timestamp() * 1000    
            if self.renko is None:
                self.renko = RenkoWS(timestampMs, cls_price, self.renko_brick_size, external_mode='nongap')
            else:
                self.renko.add_prices(timestampMs, cls_price)
                
        # Print the current time
        self.print_current_state()
            
        # Update the account state
        reward = self.get_reward(commission, include_pl=False)
        self.rewards_history.append(reward)

        # Update the step and returns
        self.current_step += 1
        
        # self.returns.append(balance - self.initial_balance)
        observation = self._get_observation()
        
        # Include the balance in the info dictionary
        info = {'balance': balance, 'pl': pl, 'position': self.position}

        # Return the required 5 values for Gymnasium
        return observation, reward, truncated, terminated, info

    def _get_observation(self):
        # Get the last 300 prices, padded if necessary
        obs = np.zeros(60, dtype=np.float32)
        
        df = None
        if self.renko:
            df = self.renko.renko_animate()
            
        balance = self.client.account.balance
        pl = self.client.account.pl

        if df is None or len(df) == 0:
            return np.append(obs, [balance, self.position, pl, self.total_commission]).astype(np.float32)
        
        # Take the last 20 prices
        df = df.tail(20)
                
        j = 0
        for i in range(len(df)):
            obs[j] = df.iloc[i]['open']
            obs[j+1] = df.iloc[i]['high']
            obs[j+2] = df.iloc[i]['low']
            
            j += 3
        
        return np.append(obs, [balance, self.position, pl, self.total_commission]).astype(np.float32)

    def reset(self, seed=None, options=None):
        super().reset(seed=seed)
        if seed is not None:
            np.random.seed(seed)
            
        self.initialize()
        
        self.render()

        # Return the initial observation and an empty info dictionary
        return self._get_observation(), {}

    def render(self, mode='human', close=False):
        balance = self.client.account.balance
        pl = self.client.account.pl
        position = self.client.account.get_position('AAPL')
        avg_reward = np.mean(self.rewards_history) if len(self.rewards_history) > 0 else 0
        print(f"Step: {self.current_step}, Tstamp: {self.timestamp}, Balance: {balance}, PL: {pl}, Position: {position}, Total Commission: {self.total_commission}, Avg Reward: {avg_reward}") 

# def load_data(csv_path):
#     df = pd.read_csv(csv_path)
    
#     # Flatten the renko_chart into a list and create corresponding price changes
#     renko_charts = []
#     prices = []
#     last_price = 100  # Start with an initial price of 100
    
#     for index, row in df.iterrows():
#         renko_sequence = list(map(int, row['renko_chart'].split(',')))
#         renko_charts.extend(renko_sequence)
#         for block in renko_sequence:
#             last_price += block * 10  # Assume renko_size=10 for simplicity
#             prices.append(last_price)
    
#     data = pd.DataFrame({
#         'renko_chart': renko_charts,
#         'price': prices
#     })
    
#     return data

# Load your data (replace 'renko_patterns.csv' with your file path)
# data = load_data('/Users/jamal/projects/slack-trading/cmd/backtester/renko_patterns.csv')
parser = argparse.ArgumentParser()
parser.add_argument('--model', type=str, help='The name of the model to load')
args = parser.parse_args()

projectsDir = os.getenv('PROJECTS_DIR')
if projectsDir is None:
    raise ValueError('PROJECTS_DIR environment variable is not set')

# Initialize the environment
env = RenkoTradingEnv(repository_source=RepositorySource.POLYGON)

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

# Add action noise for exploration
# n_actions = env.action_space.shape[-1]
# action_noise = NormalActionNoise(mean=np.zeros(n_actions), sigma=0.1 * np.ones(n_actions))

# Epsilon-greedy parameters
timestep_epsilon = 1.0  # Initial exploration rate
epsilon_min = 0.1  # Minimum exploration rate
timestep_epsilon_decay = 0.99 
epsilon_decay = 0.999  # Decay rate for exploration

# Training loop with epsilon-greedy strategy
total_timesteps = 100

# batch_size = 500  # Collect experiences in batches
obs = vec_env.reset()

for timestep in range(total_timesteps):
    epsilon = timestep_epsilon
    
    isDone = False
    time_elasped = timedelta(0)
    batch_size = 0
    while not isDone:
        batch_size += 1
        if random.random() < epsilon:
            # Take a random action

            action = [env.action_space.sample()]
        else:
            # Take the best-known action
            action, _states = model.predict(obs)
        
        # Perform the action in the environment
        obs, rewards, dones, info = vec_env.step(action)
        
        if env.client is not None:
            time_delta = env.client.time_elapsed() - time_elasped
            if time_delta >= timedelta(weeks=1):
                # Train the model with the new experience
                print(f'Training model after one week with batch size: {batch_size} ...')
                
                model.learn(total_timesteps=batch_size, reset_num_timesteps=False)
                
                # Print the current timestep and balance
                print(f'Training complete. Timestep: {timestep}, Balance: {info[0]["balance"]}')
                
                time_elasped = env.client.time_elapsed()
                
                batch_size = 0
        
        isDone = any(dones)
        
        # Decay the epilson
        if epsilon > epsilon_min:
            epsilon *= epsilon_decay
        
    # Decay the timestep epsilon
    if timestep_epsilon > epsilon_min:
        timestep_epsilon *= timestep_epsilon_decay
            
    # Train the model with the new experience
    print(f'Training model with batch size: {batch_size} ...')
    
    model.learn(total_timesteps=batch_size, reset_num_timesteps=False)
    
    # Print the current timestep and balance
    print(f'Training complete. Timestep: {timestep}, Balance: {info[0]["balance"]}')
    
    # Reset the environment
    if isDone:
        print('Resetting environment ...')
        obs = vec_env.reset()
        
    print('*' * 50)

# Save the trained model with timestamp
saveModelDir = os.path.join(projectsDir, 'slack-trading', 'cmd', 'backtester', 'models')
modelName = 'ppo_model_v3-' + datetime.now().strftime('%Y-%m-%d-%H-%M-%S')
model.save(os.path.join(saveModelDir, modelName))
print(f'Saved model: {modelName} to {saveModelDir}')

# Test the trained agent and track balance over time
env.set_repository(RepositorySource.CSV, 'validation_data.csv')
obs = vec_env.reset()
balance_over_time = []

print('Testing the agent! ...')
print('Playground ID:', env.client.id)

isDone = False
counter = 0
while not isDone:
    action, _states = model.predict(obs)
    result = vec_env.step(action)
    
    if len(result) == 5:
        obs, reward, terminated, truncated, info = result
    elif len(result) == 4:
        obs, reward, terminated, info = result
        truncated = False
    else:
        raise ValueError('Invalid result length, expected 4 or 5 values, got', len(result))

    isDone = terminated or truncated

    # Access the balance from the `info` dictionary
    balance = info[0]['balance']  # Access balance for the first environment
    balance_over_time.append(balance)

    # Access the render method for the first environment inside the DummyVecEnv
    if counter % 60 == 0:
        vec_env.env_method('render', indices=0)
    
    counter += 1


# Plot the agent's balance over time
plt.plot(balance_over_time)
plt.xlabel('Time Step')
plt.ylabel('Balance')
plt.title('Agent Balance Over Time')
plt.show()
