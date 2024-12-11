import argparse
import logging
import os
from stable_baselines3 import PPO
from stable_baselines3.common.vec_env import DummyVecEnv
from proximal_policy_optimization_v13 import TradingEnv, OrderSide, RepositorySource
# from playground_environment import PlaygroundEnvironment, RepositorySource, PlaygroundEnvironmentMode
from plotly.subplots import make_subplots
import plotly.graph_objects as go
import pandas as pd

parser = argparse.ArgumentParser()
parser.add_argument('--model', type=str, help='The name of the model to load', required=True)
parser.add_argument('--symbol', type=str, help='The symbol to backtest, e.g. COIN', required=True)
parser.add_argument('--start-date', type=str, help='The start date of the backtest in YYYY-MM-DD format', required=True)
parser.add_argument('--end-date', type=str, help='The end date of the backtest in YYYY-MM-DD format', required=True)
parser.add_argument('--host', type=str, help='The grpc host of the backtester playground', default='localhost:50051')

args = parser.parse_args()

projectsDir = os.getenv('PROJECTS_DIR')
if projectsDir is None:
    raise ValueError('PROJECTS_DIR environment variable is not set')

loadModelDir = os.path.join(projectsDir, 'slack-trading', 'cmd', 'backtester', 'models')
model = PPO.load(os.path.join(loadModelDir, args.model))

# Create a logger and add the queue handler to it
logger = logging.getLogger(__name__)
logger.setLevel(logging.DEBUG)
# Create a StreamHandler to log messages to the console
console_handler = logging.StreamHandler()
console_handler.setLevel(logging.DEBUG)  # Set the logging level for the handler

# Create a formatter and set it for the handler
formatter = logging.Formatter('%(asctime)s - %(levelname)s - %(message)s')
console_handler.setFormatter(formatter)

# Add the handler to the logger
logger.addHandler(console_handler)

# Initialize the environment
env = TradingEnv(args.symbol, args.start_date, args.end_date, args.host, logger, initial_balance=10000, repository_source=RepositorySource.POLYGON, is_training=False)

# Wrap the environment with DummyVecEnv for compatibility with Stable-Baselines3
vec_env = DummyVecEnv([lambda: env])

# Evaluate the model
obs = vec_env.reset()

# Grab the playground id
playground_id = env.client.id
 
isDone = False
rewards_series = []
data = pd.DataFrame()
while not isDone:
    action, _states = model.predict(obs)
    obs, rewards, isDone, info = vec_env.step(action)
    rewards_series.append(rewards)
    if len(info) > 0:
        timestamp = info[0]['timestamp']
        equity = info[0]['equity']
        data = data.append({'timestamp': timestamp, 'equity': equity}, ignore_index=True)

print('Results:')
print(f'Average reward: {sum(rewards_series) / len(rewards_series)}')
print(f'Min reward: {min(rewards_series)}')
print(f'Max reward: {max(rewards_series)}')
print(f'Playground ID: {playground_id}')

# Plot the equity curve
fig = make_subplots(rows=1, cols=1, shared_xaxes=True, vertical_spacing=0.2)

# Add line graph for Equity
fig.add_trace(go.Scatter(
    x=data['timestamp'],
    y=data['equity'],
    mode='lines',
    name='Equity',
    line=dict(color='blue')
))

fig.update_layout(title=f'{args.model} {args.symbol} equity from {args.start_date} to {args.end_date}',
                  xaxis_title='Date',
                  yaxis_title='Equity')

fig.show()
