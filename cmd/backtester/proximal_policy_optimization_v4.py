

import random
from datetime import timedelta
from collections import deque
import numpy as np

class ReplayBuffer:
    def __init__(self, max_size):
        self.buffer = deque(maxlen=max_size)
    
    def add(self, experience):
        self.buffer.append(experience)
    
    def sample(self, batch_size):
        indices = np.random.choice(len(self.buffer), batch_size, replace=False)
        batch = [self.buffer[idx] for idx in indices]
        states, actions, rewards, next_states, dones = map(np.array, zip(*batch))
        return states, actions, rewards, next_states, dones
    
    def size(self):
        return len(self.buffer)

# Initialize replay buffer
replay_buffer = ReplayBuffer(max_size=10000)

# Initialize variables
time_elapsed = 0
epsilon = 0.1  # Exploration rate
batch_size = 64  # Number of experiences to sample for training

while not done:
    if random.random() < epsilon:
        # Take a random action
        action = [env.action_space.sample()]
    else:
        # Take the best-known action
        action, _states = model.predict(obs)

    # Perform the action in the environment
    next_obs, rewards, dones, info = vec_env.step(action)

    # Store the experience in the replay buffer
    replay_buffer.add((obs, action, rewards, next_obs, dones))

    # Update the current observation
    obs = next_obs

    # Update time elapsed
    if env.client is not None:
        time_delta = env.client.time_elapsed() - time_elapsed
        if time_delta >= timedelta(weeks=1):
            week_has_elapsed = True
            time_elapsed = env.client.time_elapsed()

    # Check if the episode is done
    isDone = any(dones)

    # Train the model if a week has elapsed and there are enough experiences in the buffer
    if week_has_elapsed and replay_buffer.size() >= batch_size:
        states, actions, rewards, next_states, dones = replay_buffer.sample(batch_size)
        
        # Assuming model.learn can take these arguments directly or you need to process them
        model.learn(states, actions, rewards, next_states, dones)
        
        week_has_elapsed = False