module.exports = {
  apps : [{
    name: 'supertrend1-aapl',
    cmd: 'src/cmd/stats/trading_engine.py',
    args: '--sl-shift 1.0 --tp-shift 0.25 --sl-buffer 0.0 --tp-buffer 0.0 --min-max-window-in-hours 20',
    autorestart: true,
    watch: false,
    instances: 1,
    max_memory_restart: '1G',
    env: {
      BALANCE: '3000',
      SYMBOL: 'AAPL',
      OPEN_STRATEGY: 'simple_open_strategy_v4',
      MODEL_UPDATE_FREQUENCY: 'daily',
      OPTIMIZER_UPDATE_FREQUENCY: 'weekly',
      N_CALLS: '30',
    },
    env_simulation: {
      PLAYGROUND_ENV: 'simulator',
      START_DATE: '2024-11-01',
      STOP_DATE: '2025-01-31',
    },
    env_paper: {
      PLAYGROUND_CLIENT_ID: 'supertrend1-aapl-paper-14',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'paper',
      TWIRP_HOST: 'http://45.77.223.21',
    },
    env_production : {
      PLAYGROUND_CLIENT_ID: 'supertrend1-aapl-margin-14',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'margin',
      TWIRP_HOST: 'http://45.77.223.21',
    },
    interpreter: 'anaconda/envs/trading/bin/python',
  },{
    name: 'supertrend1-coin',
    cmd: 'src/cmd/stats/trading_engine.py',
    args: '--sl-shift 1.0 --tp-shift 0.25 --sl-buffer 0.0 --tp-buffer 0.0 --min-max-window-in-hours 20',
    autorestart: true,
    watch: false,
    instances: 1,
    max_memory_restart: '1G',
    env: {
      BALANCE: '3000',
      SYMBOL: 'COIN',
      TWIRP_HOST: 'http://localhost:5051',
      OPEN_STRATEGY: 'simple_open_strategy_v4',
      MODEL_UPDATE_FREQUENCY: 'daily',
      OPTIMIZER_UPDATE_FREQUENCY: 'weekly',
      N_CALLS: '30',
    },
    env_paper: {
      PLAYGROUND_CLIENT_ID: 'supertrend1-coin-paper-14',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'paper',
      TWIRP_HOST: 'http://45.77.223.21',
    },
    env_production : {
      PLAYGROUND_CLIENT_ID: 'supertrend1-coin-margin-14',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'margin',
      TWIRP_HOST: 'http://45.77.223.21',
    },
    interpreter: 'anaconda/envs/trading/bin/python',
  },{
    name: 'supertrend1-meta',
    cmd: 'src/cmd/stats/trading_engine.py',
    args: '--sl-shift 1.0 --tp-shift 0.25 --sl-buffer 0.0 --tp-buffer 0.0 --min-max-window-in-hours 20',
    autorestart: true,
    watch: false,
    instances: 1,
    max_memory_restart: '1G',
    env: {
      BALANCE: '3000',
      SYMBOL: 'META',
      TWIRP_HOST: 'http://localhost:5051',
      OPEN_STRATEGY: 'simple_open_strategy_v4',
      MODEL_UPDATE_FREQUENCY: 'daily',
      OPTIMIZER_UPDATE_FREQUENCY: 'weekly',
      N_CALLS: '30',
    },
    env_simulation: {
      PLAYGROUND_ENV: 'simulator',
      START_DATE: '2024-11-01',
      STOP_DATE: '2025-01-31',
    },
    env_paper: {
      PLAYGROUND_CLIENT_ID: 'supertrend1-meta-paper-14',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'paper',
      TWIRP_HOST: 'http://45.77.223.21',
    },
    env_production : {
      PLAYGROUND_CLIENT_ID: 'supertrend1-meta-margin-14',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'margin',
      TWIRP_HOST: 'http://45.77.223.21',
    },
    interpreter: 'anaconda/envs/trading/bin/python',
  },{
    name: 'supertrend1-nvda',
    cmd: 'src/cmd/stats/trading_engine.py',
    args: '--sl-shift 1.0 --tp-shift 0.25 --sl-buffer 0.0 --tp-buffer 0.0 --min-max-window-in-hours 20',
    autorestart: true,
    watch: false,
    instances: 1,
    max_memory_restart: '1G',
    env: {
      BALANCE: '3000',
      SYMBOL: 'NVDA',
      TWIRP_HOST: 'http://localhost:5051',
      OPEN_STRATEGY: 'simple_open_strategy_v4',
      MODEL_UPDATE_FREQUENCY: 'daily',
      OPTIMIZER_UPDATE_FREQUENCY: 'weekly',
      N_CALLS: '30',
    },
    env_simulation: {
      PLAYGROUND_ENV: 'simulator',
      START_DATE: '2025-05-28',
      STOP_DATE: '2025-05-29',
      TWIRP_HOST: 'http://45.77.223.21',
    },
    env_paper: {
      PLAYGROUND_CLIENT_ID: 'supertrend1-nvda-paper-14',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'paper',
      TWIRP_HOST: 'http://45.77.223.21',
    },
    env_production : {
      PLAYGROUND_CLIENT_ID: 'supertrend1-nvda-margin-14',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'margin',
      TWIRP_HOST: 'http://45.77.223.21',
    },
    interpreter: 'anaconda/envs/trading/bin/python',
  },{
    name: 'supertrend1-tsla',
    cmd: 'src/cmd/stats/trading_engine.py',
    args: '--sl-shift 1.0 --tp-shift 0.25 --sl-buffer 0.0 --tp-buffer 0.0 --min-max-window-in-hours 20',
    autorestart: true,
    watch: false,
    instances: 1,
    max_memory_restart: '1G',
    env: {
      BALANCE: '3000',
      SYMBOL: 'TSLA',
      TWIRP_HOST: 'http://localhost:5051',
      OPEN_STRATEGY: 'simple_open_strategy_v4',
      MODEL_UPDATE_FREQUENCY: 'daily',
      OPTIMIZER_UPDATE_FREQUENCY: 'weekly',
      N_CALLS: '30',
    },
    env_simulation: {
      PLAYGROUND_ENV: 'simulator',
      START_DATE: '2024-11-01',
      STOP_DATE: '2025-01-31',
    },
    env_paper: {
      PLAYGROUND_CLIENT_ID: 'supertrend1-tsla-paper-14',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'paper',
      TWIRP_HOST: 'http://45.77.223.21',
    },
    env_production : {
      PLAYGROUND_CLIENT_ID: 'supertrend1-tsla-margin-14',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'margin',
      TWIRP_HOST: 'http://45.77.223.21',
    },
    interpreter: 'anaconda/envs/trading/bin/python',
  }]
};