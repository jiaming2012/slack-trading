module.exports = {
  apps : [{
    name: 'stacked-strategy',
    cmd: 'src/cmd/stats/trading_engine.py',
    args: '--max-open-count 3 --target-risk-to-reward 1.9 --max-per-trade-risk-percentage 0.06',
    autorestart: true,
    watch: false,
    instances: 1,
    max_memory_restart: '2G',
    env: {
      BALANCE: '20000',
      SYMBOL: 'TSLA COIN NVDA AAPL META',
      OPEN_STRATEGY: 'simple_stack_open_strategy_v1',
      MODEL_UPDATE_FREQUENCY: 'daily',
      OPTIMIZER_UPDATE_FREQUENCY: 'weekly',
      TWIRP_HOST: 'http://45.77.223.21',
      PLAYGROUND_CLIENT_ID: 'stacked-strategy-2',
      N_CALLS: '30',
    },
    env_simulation: {},
    env_paper: {
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'paper',
    },
    env_production : {
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'margin',
    },
    interpreter: 'anaconda/envs/trading/bin/python',
  }]
};