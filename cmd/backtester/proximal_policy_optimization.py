import gymnasium as gym
from gymnasium import spaces
import numpy as np
import pandas as pd
from stable_baselines3 import PPO
from stable_baselines3.common.vec_env import DummyVecEnv
from datetime import datetime
import random
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
        self.current_step = 0
        self.position = 0  # 1 for long, -1 for short, 0 for no position
        self.playground_client = BacktesterPlaygroundClient(self.balance, 'AAPL', '2021-01-04', '2021-01-12', RepositorySource.CSV, 'training_data.csv')
        self.returns = []
        self.negative_returns = []
        self.recent_close_prices = []
        self.is_backtest_complete = False
        self.sl = 0
        self.tp = 0
        self.pl = 0
        self.current_price = 0
        self.ma = 0
        self.timestamp = None
        self._internal_timestamp = None
        self.total_commission = 0
        self.sl_history, self.tp_history, self.rewards_history = [], [], []
        
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
        self.sl_history, self.tp_history, self.rewards_history = [], [], []

        # Action space: Continuous (take_profit, stop_loss)
        self.action_space = spaces.Box(low=np.array([0, 0]), high=np.array([100, 100]), dtype=np.float32)

        # Observation space: Last 10 Renko blocks + portfolio balance + pl + position
        self.observation_space = spaces.Box(low=-np.inf, high=np.inf, shape=(6,), dtype=np.float32)
        
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
            avg_sl = np.mean(self.sl_history) if len(self.sl_history) > 0 else 0
            avg_tp = np.mean(self.tp_history) if len(self.tp_history) > 0 else 0
            avg_reward = np.mean(self.rewards_history) if len(self.rewards_history) > 0 else 0
            
            print(f'Current time: {self._internal_timestamp}, Balance: {self.balance}, Position: {self.position}, PL: {self.pl}, Current Price: {self.current_price}, Avg SL: {avg_sl}, Avg TP: {avg_tp}, Avg Reward: {avg_reward}')

    def get_reward(self):
        return self.balance + self.pl - self.initial_balance - self.total_commission
    
    def step(self, action):
        # Example custom logic to apply the action and calculate reward
        # renko_size = action[0]
        stop_loss = action[0]
        take_profit = action[1]
        
        self.sl = stop_loss
        self.tp = take_profit
        
        self.sl_history.append(self.sl)
        self.tp_history.append(self.tp)

        terminated = False
        truncated = False
        if self.balance <= 0:
            reward = self.get_reward()
            self.rewards_history.append(reward)
            terminated = True
            self._get_observation(), reward, terminated, truncated, {'balance': self.balance, 'pl': self.pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }

        # Ensure we are still within the data bounds
        if self.playground_client.is_backtest_complete():
            reward = self.get_reward()
            self.rewards_history.append(reward)
            truncated = True  # Episode truncated (e.g., max steps reached)
            return self._get_observation(), reward, terminated, truncated, {'balance': self.balance, 'pl': self.pl, 'position': self.position, 'current_price': self.current_price, 'ma': self.ma }

        # Simulate trade, adjust balance, and calculate reward
        account = self.playground_client.fetch_account_state()
        commission = 0
         
        if self.position == 0:
            if len(self.recent_close_prices) >= 5:
                non_zero_prices = [price for price in self.recent_close_prices if price != 0]
                self.ma = np.mean(non_zero_prices)
                self.current_price = non_zero_prices[-1]
                
                if self.current_price > self.ma:
                    self.playground_client.place_order('AAPL', 1, OrderSide.BUY)
                    self.position = 1
                    commission = 2
                elif self.current_price < self.ma:
                    self.playground_client.place_order('AAPL', 1, OrderSide.SELL_SHORT)
                    self.position = -1
                    commission = 2
                    
            self.pl = 0
        else:        
            self.pl  = account['positions']['AAPL']['pl']
            
            if self.pl >= take_profit:
                # Close the position
                if self.position > 0:
                    self.playground_client.place_order('AAPL', 1, OrderSide.SELL)
                    self.position = 0
                elif self.position < 0:
                    self.playground_client.place_order('AAPL', 1, OrderSide.BUY_TO_COVER)
                    self.position = 0
                                        
            elif self.pl <= -stop_loss:
                # Close the position
                if self.position > 0:
                    self.playground_client.place_order('AAPL', 1, OrderSide.SELL)
                    self.position = 0
                elif self.position < 0:
                    self.playground_client.place_order('AAPL', 1, OrderSide.BUY_TO_COVER)
                    self.position = 0
                                    
        self.total_commission += commission
        
        # Move into the future by one step
        current_state = self.playground_client.tick(60)
        # reward = self.playground_client.fetch_reward_from_new_trades(current_state, stop_loss, take_profit, commission)
        
        if self.playground_client.current_candle:
            self.timestamp = self.playground_client.current_candle['datetime']
            self.recent_close_prices.append(self.playground_client.current_candle['close'])
            if len(self.recent_close_prices) > 100:
                self.recent_close_prices.pop(0)
                
        # Print the current time
        self.print_current_state()
            
        # Update the account state
        account = self.playground_client.fetch_account_state()
        self.balance = account['balance'] - self.total_commission
        reward = self.get_reward()
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
        # Get the last 10 prices, padded if necessary
        obs = np.zeros(0, dtype=np.float32)
        
        # Ensure that the renko_chart has enough data to fill the observation
        # renko_blocks = self.data['renko_chart'].iloc[self.current_step:self.current_step + 10].values
        # obs[:len(renko_blocks)] = renko_blocks
        if len(self.recent_close_prices) > 0:
            moving_average = np.mean(self.recent_close_prices)
            min = np.min(self.recent_close_prices)
            max = np.max(self.recent_close_prices)
            
            # obs[:len(self.close_prices)] = self.close_prices - moving_average
            obs = np.append(obs, [self.recent_close_prices[-1]]).astype(np.float32)
            obs = np.append(obs, [moving_average]).astype(np.float32)
            obs = np.append(obs, [min]).astype(np.float32)
            obs = np.append(obs, [max]).astype(np.float32)
        else:
            obs = np.append(obs, [0]).astype(np.float32)
            obs = np.append(obs, [0]).astype(np.float32)
            obs = np.append(obs, [0]).astype(np.float32)
            obs = np.append(obs, [0]).astype(np.float32)

        # Append the balance as the 11th element
        # obs = np.append(obs, [self.balance]).astype(np.float32)
        
        # Append pl as the 12th element
        obs = np.append(obs, [self.pl]).astype(np.float32)
        
        # Append position as the 13th element
        obs = np.append(obs, [self.position]).astype(np.float32)
        
        return obs

    def reset(self, seed=None, options=None):
        super().reset(seed=seed)
        if seed is not None:
            np.random.seed(seed)
            
        print(f'Reset called (seed={seed}): balance={self.balance}, position={self.position}, pl={self.pl}, timestamp={self.timestamp}')
        
        self.initialize()
        

        # Return the initial observation and an empty info dictionary
        return self._get_observation(), {}

    def render(self, mode='human', close=False):
        avg_sl = np.mean(self.sl_history) if len(self.sl_history) > 0 else 0
        avg_tp = np.mean(self.tp_history) if len(self.tp_history) > 0 else 0
        avg_reward = np.mean(self.rewards_history) if len(self.rewards_history) > 0 else 0
        
        print(f"Step: {self.current_step}, Tstamp: {self.timestamp}, Balance: {self.balance}, Position: {self.position}, SL: {self.sl}, TP: {self.tp}, Total Commission: {self.total_commission}, Avg SL: {avg_sl}, Avg TP: {avg_tp}, Avg Reward: {avg_reward}") 

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

# Initialize the environment
env = RenkoTradingEnv()

# Wrap the environment with DummyVecEnv for compatibility with Stable-Baselines3
vec_env = DummyVecEnv([lambda: env])

# Add action noise for exploration
# n_actions = env.action_space.shape[-1]
# action_noise = NormalActionNoise(mean=np.zeros(n_actions), sigma=0.1 * np.ones(n_actions))

# Create and train the PPO model
model = PPO('MlpPolicy', vec_env, verbose=1, policy_kwargs={'net_arch': [128, 128]}, ent_coef=0.1)

# Epsilon-greedy parameters
timestep_epsilon = 1.0  # Initial exploration rate
epsilon_min = 0.1  # Minimum exploration rate
timestep_epsilon_decay = 0.9 
epsilon_decay = 0.999  # Decay rate for exploration

# Training loop with epsilon-greedy strategy
total_timesteps = 5
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

# Test the trained agent and track balance over time
obs = vec_env.reset()
balance_over_time = []

print('Testing the agent! ...')
print('Playground ID:', env.playground_client.id)

isDone = False
while not isDone:
    action, _states = model.predict(obs)
    result = vec_env.step(action)
    sl = action[0][0]
    tp = action[0][1]
    
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







