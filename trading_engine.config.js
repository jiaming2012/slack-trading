module.exports = {
  apps : [{
    name: 'supertrend1-aapl',
    cmd: '/Users/jamal/projects/slack-trading/src/cmd/stats/trading_engine.py',
    args: '--sl-shift -8.35 --tp-shift 10.0 --sl-buffer 4.11 --tp-buffer 4.78 --min-max-window-in-hours 6',
    autorestart: true,
    watch: true,
    instances: 1,
    max_memory_restart: '1G',
    env_dev: {
      BALANCE: '10000',
      SYMBOL: 'AAPL',
      GRPC_HOST: 'http://45.77.223.21',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'paper',
      OPEN_STRATEGY: 'simple_open_strategy_v1',
      MODEL_UPDATE_FREQUENCY: 'weekly',
    },
    env_production : {
      ENV: 'production'
    },
    interpreter: '/Users/jamal/projects/slack-trading/src/cmd/stats/env/bin/python',
  },{
    name: 'supertrend1-coin',
    cmd: '/Users/jamal/projects/slack-trading/src/cmd/stats/trading_engine.py',
    args: '--sl-shift -4.84 --tp-shift 2.88 --sl-buffer 1.49 --tp-buffer 2.18 --min-max-window-in-hours 16',
    autorestart: true,
    watch: true,
    instances: 1,
    max_memory_restart: '1G',
    env_dev: {
      BALANCE: '10000',
      SYMBOL: 'COIN',
      GRPC_HOST: 'http://45.77.223.21',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'paper',
      OPEN_STRATEGY: 'simple_open_strategy_v1',
      MODEL_UPDATE_FREQUENCY: 'weekly',
    },
    env_production : {
      ENV: 'production'
    },
    interpreter: '/Users/jamal/projects/slack-trading/src/cmd/stats/env/bin/python',
  },{
    name: 'supertrend1-meta',
    cmd: '/Users/jamal/projects/slack-trading/src/cmd/stats/trading_engine.py',
    args: '--sl-shift 8.96 --tp-shift 10.0 --sl-buffer 2.69 --tp-buffer 5.0 --min-max-window-in-hours 5',
    autorestart: true,
    watch: true,
    instances: 1,
    max_memory_restart: '1G',
    env_dev: {
      BALANCE: '10000',
      SYMBOL: 'META',
      GRPC_HOST: 'http://45.77.223.21',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'paper',
      OPEN_STRATEGY: 'simple_open_strategy_v1',
      MODEL_UPDATE_FREQUENCY: 'weekly',
    },
    env_production : {
      ENV: 'production'
    },
    interpreter: '/Users/jamal/projects/slack-trading/src/cmd/stats/env/bin/python',
  },{
    name: 'supertrend1-nvda',
    cmd: '/Users/jamal/projects/slack-trading/src/cmd/stats/trading_engine.py',
    args: '--sl-shift 9.54 --tp-shift 1.54 --sl-buffer 2.34 --tp-buffer 4.27 --min-max-window-in-hours 12',
    autorestart: true,
    watch: true,
    instances: 1,
    max_memory_restart: '1G',
    env_dev: {
      BALANCE: '10000',
      SYMBOL: 'NVDA',
      GRPC_HOST: 'http://45.77.223.21',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'paper',
      OPEN_STRATEGY: 'simple_open_strategy_v1',
      MODEL_UPDATE_FREQUENCY: 'weekly',
    },
    env_production : {
      ENV: 'production'
    },
    interpreter: '/Users/jamal/projects/slack-trading/src/cmd/stats/env/bin/python',
  },{
    name: 'supertrend1-tsla',
    cmd: '/Users/jamal/projects/slack-trading/src/cmd/stats/trading_engine.py',
    args: '--sl-shift -3.05 --tp-shift -3.8 --sl-buffer 3.1 --tp-buffer 4.76 --min-max-window-in-hours 11',
    autorestart: true,
    watch: true,
    instances: 1,
    max_memory_restart: '1G',
    env_dev: {
      BALANCE: '10000',
      SYMBOL: 'TSLA',
      GRPC_HOST: 'http://45.77.223.21',
      PLAYGROUND_ENV: 'live',
      LIVE_ACCOUNT_TYPE: 'paper',
      OPEN_STRATEGY: 'simple_open_strategy_v1',
      MODEL_UPDATE_FREQUENCY: 'weekly',
    },
    env_production : {
      ENV: 'production'
    },
    interpreter: '/Users/jamal/projects/slack-trading/src/cmd/stats/env/bin/python',
  }]
};