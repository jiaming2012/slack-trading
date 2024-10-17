import gymnasium as gym
from gymnasium import spaces
import numpy as np
import pandas as pd
from stable_baselines3 import PPO
from stable_baselines3.common.vec_env import DummyVecEnv
from datetime import datetime
import random
import argparse
import os

# from stable_baselines3.common.noise import NormalActionNoise
import matplotlib.pyplot as plt
from backtester_playground_client import BacktesterPlaygroundClient, OrderSide, RepositorySource

class RenkoTradingEnv(gym.Env):
    """
    Custom Environment for Renko Chart Trading using PPO and Sortino Ratio as reward.
    """
    metadata = {'render.modes': ['human']}
    
    def initialize(self):
        self.balance = self.initial_balance
        self.previous_balance = self.balance
        self.current_step = 0
        self.position = 0  # 1 for long, -1 for short, 0 for no position
        self.playground_client = BacktesterPlaygroundClient(self.balance, 'AAPL', '2021-01-04', '2021-01-12', RepositorySource.CSV, 'training_data.csv')
        self.returns = []
        self.negative_returns = []
        self.recent_close_prices = np.array([])
        self.is_backtest_complete = False
        self.sl = 0
        self.tp = 0
        self.pl = 0
        self.current_price = 0
        self.ma = 0
        self.timestamp = None
        self._internal_timestamp = None
        self.total_commission = 0
        self.rewards_history = []
        
        print(f'Running simulation in playground {self.playground_client.id}')
        
    def __init__(self, initial_balance=10000):
        super(RenkoTradingEnv, self).__init__()
        
        # Parameters and variables
        self.initial_balance = initial_balance
        self.balance = None
        self.position = None
        self.pl = None
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
        self.observation_space = spaces.Box(low=-np.inf, high=np.inf, shape=(305,), dtype=np.float32)
        
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
            avg_reward = np.mean(self.rewards_history) if len(self.rewards_history) > 0 else 0
            
            print(f'Current time: {self._internal_timestamp}, Balance: {self.balance}, Commission: {self.total_commission}, PL: {self.pl}, Current Price: {self.current_price}, Avg Reward: {avg_reward}')

    def get_reward(self, commission):
        result = self.balance - self.previous_balance - commission
        self.previous_balance = self.balance
        return result
    
    def step(self, action):
        # Example custom logic to apply the action and calculate reward
        # renko_size = action[0]
        position = round(action[0])  # Discrete action as integer

        terminated = False
        truncated = False
        if self.balance <= 0:
            reward = self.get_reward(0)
            self.rewards_history.append(reward)
            terminated = True
            self._get_observation(), reward, terminated, truncated, {'balance': self.balance, 'pl': self.pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }

        # Ensure we are still within the data bounds
        if self.playground_client.is_backtest_complete():
            reward = self.get_reward(0)
            self.rewards_history.append(reward)
            truncated = True  # Episode truncated (e.g., max steps reached)
            return self._get_observation(), reward, terminated, truncated, {'balance': self.balance, 'pl': self.pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }

        # Simulate trade, adjust balance, and calculate reward
        account = self.playground_client.fetch_and_update_account_state()
        if account['positions'].get('AAPL'):
            self.pl  = account['positions']['AAPL']['pl']
        else: 
            self.pl = 0
        
        commission = 0
        
        seconds_elapsed = 60
         
        # if len(self.recent_close_prices) >= 5:
        if position > self.position >= 0:
            self.playground_client.place_order('AAPL', position, OrderSide.BUY)
            
            cs = self.playground_client.tick(1)
                
            if self.playground_client.is_backtest_complete():
                reward = self.get_reward(0)
                self.rewards_history.append(reward)
                truncated = True  # Episode truncated (e.g., max steps reached)
                return self._get_observation(), reward, terminated, truncated, {'balance': self.balance, 'pl': self.pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }
            
            seconds_elapsed -= 1
                
            commission = 2 * position
            self.position += position
        elif position < self.position <= 0:
            self.playground_client.place_order('AAPL', abs(position), OrderSide.SELL_SHORT)
            
            cs = self.playground_client.tick(1)
                
            if self.playground_client.is_backtest_complete():
                reward = self.get_reward(0)
                self.rewards_history.append(reward)
                truncated = True  # Episode truncated (e.g., max steps reached)
                return self._get_observation(), reward, terminated, truncated, {'balance': self.balance, 'pl': self.pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }
            
            seconds_elapsed -= 1
            
            commission = 2 * position
            self.position += position
        elif position < 0 and self.position > 0:
            # close positive position
            self.playground_client.place_order('AAPL', self.position, OrderSide.SELL)
            cs = self.playground_client.tick(1)
            
            if self.playground_client.is_backtest_complete():
                reward = self.get_reward(0)
                self.rewards_history.append(reward)
                truncated = True  # Episode truncated (e.g., max steps reached)
                return self._get_observation(), reward, terminated, truncated, {'balance': self.balance, 'pl': self.pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }
                
            seconds_elapsed -= 1
            
            # open new short position
            try:
                self.playground_client.place_order('AAPL', abs(position), OrderSide.SELL_SHORT)
                
                cs = self.playground_client.tick(1)
                
                if self.playground_client.is_backtest_complete():
                    reward = self.get_reward(0)
                    self.rewards_history.append(reward)
                    truncated = True  # Episode truncated (e.g., max steps reached)
                    return self._get_observation(), reward, terminated, truncated, {'balance': self.balance, 'pl': self.pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }
                
                seconds_elapsed -= 1
            except Exception as e:
                print('cs: ', cs)
                raise(e)
            
            commission = 2 * abs(position)
            self.position = position
        elif position > 0 and self.position < 0:
            # close negative position
            self.playground_client.place_order('AAPL', abs(self.position), OrderSide.BUY_TO_COVER)
            cs = self.playground_client.tick(1)
            
            if self.playground_client.is_backtest_complete():
                reward = self.get_reward(0)
                self.rewards_history.append(reward)
                truncated = True  # Episode truncated (e.g., max steps reached)
                return self._get_observation(), reward, terminated, truncated, {'balance': self.balance, 'pl': self.pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }
                
            seconds_elapsed -= 1
            
            # open new long position
            try:
                self.playground_client.place_order('AAPL', position, OrderSide.BUY)
                cs = self.playground_client.tick(1)
                
                if self.playground_client.is_backtest_complete():
                    reward = self.get_reward(0)
                    self.rewards_history.append(reward)
                    truncated = True  # Episode truncated (e.g., max steps reached)
                    return self._get_observation(), reward, terminated, truncated, {'balance': self.balance, 'pl': self.pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }
                
                seconds_elapsed -= 1
            
            except Exception as e:
                print('cs: ', cs)
                raise(e)
            
            commission = 2 * position
            self.position = position
    
                                    
        self.total_commission += commission
        
        # Move into the future by one step
        current_state = self.playground_client.tick(seconds_elapsed)
        # reward = self.playground_client.fetch_reward_from_new_trades(current_state, stop_loss, take_profit, commission)
        
        if self.playground_client.current_candle:
            self.timestamp = self.playground_client.current_candle['datetime']
            self.recent_close_prices = np.append(self.recent_close_prices, self.playground_client.current_candle['close'])
            if len(self.recent_close_prices) > 300:
                self.recent_close_prices = self.recent_close_prices[1:]  # Remove the first element
                
        # Print the current time
        self.print_current_state()
            
        # Update the account state
        account = self.playground_client.fetch_and_update_account_state()
        self.balance = account['balance']
        reward = self.get_reward(commission)
        self.rewards_history.append(reward)
        

        # Update the step and returns
        self.current_step += 1
        # self.returns.append(self.balance - self.initial_balance)
        observation = self._get_observation()
        
        # Include the balance in the info dictionary
        info = {'balance': self.balance, 'pl': self.pl, 'position': self.position}

        # Return the required 5 values for Gymnasium
        return observation, reward, truncated, terminated, info

    def _get_observation(self):
        # Get the last 300 prices, padded if necessary
        obs = np.zeros(300, dtype=np.float32)

        if len(self.recent_close_prices) == 0:
            return np.append(obs, [self.balance, self.position, self.pl, self.initial_balance, self.total_commission]).astype(np.float32)
        
        # Create a non-zero mask
        non_zero_mask = self.recent_close_prices != 0

        mean = np.mean(self.recent_close_prices[non_zero_mask])

        diff = self.recent_close_prices - mean

        obs[:len(diff)] = diff
        
        return np.append(obs, [self.balance, self.position, self.pl, self.initial_balance, self.total_commission]).astype(np.float32)

    def reset(self, seed=None, options=None):
        super().reset(seed=seed)
        if seed is not None:
            np.random.seed(seed)
            
        print(f'Reset called (seed={seed}): balance={self.balance}, position={self.position}, pl={self.pl}, timestamp={self.timestamp}')
        
        self.initialize()
        

        # Return the initial observation and an empty info dictionary
        return self._get_observation(), {}

    def render(self, mode='human', close=False):
        avg_reward = np.mean(self.rewards_history) if len(self.rewards_history) > 0 else 0
        print(f"Step: {self.current_step}, Tstamp: {self.timestamp}, Balance: {self.balance}, Commission: {self.total_commission}, SL: {self.sl}, TP: {self.tp}, Total Commission: {self.total_commission}, Avg Reward: {avg_reward}") 

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
env = RenkoTradingEnv()

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
    model = PPO('MlpPolicy', vec_env, verbose=1, policy_kwargs={'net_arch': [256, 128, 64]}, ent_coef=0.5, learning_rate=0.001)

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
    obs = vec_env.reset()

# Save the trained model with timestamp
saveModelDir = os.path.join(projectsDir, 'slack-trading', 'cmd', 'backtester', 'models')
modelName = 'ppo_model_v2-' + datetime.now().strftime('%Y-%m-%d-%H-%M-%S')
model.save(os.path.join(saveModelDir, modelName))
print(f'Saved model: {modelName} to {saveModelDir}')

# Test the trained agent and track balance over time
obs = vec_env.reset()
balance_over_time = []

print('Testing the agent! ...')
print('Playground ID:', env.playground_client.id)

isDone = False
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
    vec_env.env_method('render', indices=0)


# Plot the agent's balance over time
plt.plot(balance_over_time)
plt.xlabel('Time Step')
plt.ylabel('Balance')
plt.title('Agent Balance Over Time')
plt.show()
