import gymnasium as gym
from gymnasium import spaces
import numpy as np
import pandas as pd
from stable_baselines3 import PPO
from stable_baselines3.common.vec_env import DummyVecEnv
import matplotlib.pyplot as plt
from backtester_playground_client import BacktesterPlaygroundClient, OrderSide, RepositorySource

class RenkoTradingEnv(gym.Env):
    """
    Custom Environment for Renko Chart Trading using PPO and Sortino Ratio as reward.
    """
    metadata = {'render.modes': ['human']}
    
    def __init__(self, initial_balance=10000):
        super(RenkoTradingEnv, self).__init__()
        
        # Parameters and variables
        # self.data = data
        self.initial_balance = initial_balance
        self.balance = initial_balance
        self.current_step = 0
        self.position = 0  # 1 for long, -1 for short, 0 for no position
        self.playground_client = BacktesterPlaygroundClient(initial_balance, 'AAPL', '2021-01-04', '2021-01-31', RepositorySource.CSV, 'training_data.csv')
        self.returns = []
        self.negative_returns = []
        self.close_prices = []
        self.is_backtest_complete = False
        self.sl = None
        self.tp = None
        self.timestamp = None
        
        print(f'Running simulation in playground {self.playground_client.id}')

        # Action space: Continuous (renko_size, take_profit, stop_loss)
        self.action_space = spaces.Box(low=np.array([5, 1, 1]), high=np.array([20, 5, 5]), dtype=np.float32)

        # Observation space: Last 10 Renko blocks + portfolio balance
        self.observation_space = spaces.Box(low=-np.inf, high=np.inf, shape=(11,), dtype=np.float32)

    def step(self, action):
        # Example custom logic to apply the action and calculate reward
        renko_size = action[0]
        take_profit = action[1]
        stop_loss = action[2]
        
        self.sl = stop_loss
        self.tp = take_profit

        reward = 0
        terminated = False
        truncated = False

        # Ensure we are still within the data bounds
        if self.playground_client.is_backtest_complete():
            truncated = True  # Episode truncated (e.g., max steps reached)
            return self._get_observation(), reward, terminated, truncated, {'balance': self.balance}

        # Simulate trade, adjust balance, and calculate reward
        account = self.playground_client.fetch_account_state()
         
        if self.position == 0:
            if len(self.close_prices) >= 10:
                ma = np.mean(self.close_prices)
                current_price = self.close_prices[-1]
                
                if current_price > ma:
                    self.playground_client.place_order('AAPL', 1, OrderSide.SELL_SHORT)
                    self.position = -1
                elif current_price < ma:
                    self.playground_client.place_order('AAPL', 1, OrderSide.BUY)
                    self.position = 1
        else:        
            pl  = account['positions']['AAPL']['pl']
            
            if pl >= take_profit:
                # Close the position
                if self.position > 0:
                    self.playground_client.place_order('AAPL', 1, OrderSide.SELL)
                    self.position = 0
                elif self.position < 0:
                    self.playground_client.place_order('AAPL', 1, OrderSide.BUY_TO_COVER)
                    self.position = 0
                    
                reward = pl
                    
            elif pl <= -stop_loss:
                # Close the position
                if self.position > 0:
                    self.playground_client.place_order('AAPL', 1, OrderSide.SELL)
                    self.position = 0
                elif self.position < 0:
                    self.playground_client.place_order('AAPL', 1, OrderSide.BUY_TO_COVER)
                    self.position = 0
                    
                reward = pl
                
        # Move into the future by one step
        self.playground_client.tick(60)
        if self.playground_client.current_candle:
            self.timestamp = self.playground_client.current_candle['datetime']
            self.close_prices.append(self.playground_client.current_candle['close'])
            if len(self.close_prices) > 10:
                self.close_prices.pop(0)
            
        # Update the account state
        account = self.playground_client.fetch_account_state()
        self.balance = account['balance']

        # Update the step and returns
        self.current_step += 1
        self.returns.append(self.balance - self.initial_balance)
        observation = self._get_observation()
        
        # Include the balance in the info dictionary
        info = {'balance': self.balance}

        # Return the required 5 values for Gymnasium
        return observation, reward, terminated, truncated, info

    def _get_observation(self):
        # Get the last 10 prices, padded if necessary
        obs = np.zeros(10, dtype=np.float32)
        
        # Ensure that the renko_chart has enough data to fill the observation
        # renko_blocks = self.data['renko_chart'].iloc[self.current_step:self.current_step + 10].values
        # obs[:len(renko_blocks)] = renko_blocks
        if len(self.close_prices) >= 10:
            moving_average = np.mean(self.close_prices)
            
            obs[:len(self.close_prices)] = self.close_prices - moving_average

        # Append the balance as the 11th element
        return np.append(obs, [self.balance]).astype(np.float32)

    def reset(self, seed=None, options=None):
        super().reset(seed=seed)
        if seed is not None:
            np.random.seed(seed)
        
        self.playground_client = BacktesterPlaygroundClient(self.initial_balance, 'AAPL', '2021-01-04', '2021-01-31', RepositorySource.CSV, 'training_data.csv')
        self.balance = self.initial_balance
        self.current_step = 0
        self.position = 0
        self.returns = []
        self.negative_returns = []
        self.close_prices = []
        self.sl = None
        self.tp = None

        # Return the initial observation and an empty info dictionary
        return self._get_observation(), {}

    def render(self, mode='human', close=False):
        print(f"Step: {self.current_step}, Tstamp: {self.timestamp}, Balance: {self.balance}, Position: {self.position}, SL: {self.sl}, TP: {self.tp}")

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

# Create and train the PPO model
model = PPO('MlpPolicy', vec_env, verbose=1, policy_kwargs={'net_arch': [64, 64]})
model.learn(total_timesteps=10000)

# Test the trained agent and track balance over time
obs = vec_env.reset()
balance_over_time = []

print('Testing the agent...')
print('Playground ID:', env.playground_client.id)

for i in range(2000):
    action, _states = model.predict(obs)
    result = vec_env.step(action)
    sl = action[0][1]
    tp = action[0][2]
    
    if len(result) == 5:
        obs, reward, terminated, truncated, info = result
    elif len(result) == 4:
        obs, reward, terminated, info = result
        truncated = False
    else:
        raise ValueError('Invalid result length, expected 4 or 5 values, got', len(result))

    done = terminated or truncated

    # Access the balance from the `info` dictionary
    balance = info[0]['balance']  # Access balance for the first environment
    balance_over_time.append(balance)

    # Access the render method for the first environment inside the DummyVecEnv
    vec_env.env_method('render', indices=0)
    
    if done:
        obs = vec_env.reset()

# Plot the agent's balance over time
plt.plot(balance_over_time)
plt.xlabel('Time Step')
plt.ylabel('Balance')
plt.title('Agent Balance Over Time')
plt.show()
