module.exports = {
  apps : [{
    name: 'optimizer-aapl',
    cmd: 'src/cmd/stats/trading_engine_optimizer.py',
    args: '',
    autorestart: false,
    watch: false,
    instances: 1,
    max_memory_restart: '1G',
    env_dev: {
      BALANCE: '10000',
      SYMBOL: 'AAPL',
      GRPC_HOST: 'http://45.77.223.21',
      PLAYGROUND_ENV: 'simulator',
      OPEN_STRATEGY: 'simple_open_strategy_v1',
      START_DATE: '2024-12-15',
      STOP_DATE: '2025-01-24',
      MODEL_UPDATE_FREQUENCY: 'weekly',
    },
    env_production : {
      ENV: 'production'
    },
    interpreter: 'src/cmd/stats/env/bin/python',
  },{
    name: 'supertrend1-coin',
    cmd: 'src/cmd/stats/trading_engine_optimizer.py',
    args: '',
    autorestart: false,
    watch: false,
    instances: 1,
    max_memory_restart: '1G',
    env_dev: {
      BALANCE: '10000',
      SYMBOL: 'COIN',
      GRPC_HOST: 'http://45.77.223.21',
      PLAYGROUND_ENV: 'simulator',
      OPEN_STRATEGY: 'simple_open_strategy_v1',
      START_DATE: '2024-12-15',
      STOP_DATE: '2025-01-24',
      MODEL_UPDATE_FREQUENCY: 'weekly',
    },
    env_production : {
      ENV: 'production'
    },
    interpreter: 'src/cmd/stats/env/bin/python',
  },{
    name: 'supertrend1-tsla',
    cmd: 'src/cmd/stats/trading_engine_optimizer.py',
    args: '',
    autorestart: false,
    watch: false,
    instances: 1,
    max_memory_restart: '1G',
    env_dev: {
      BALANCE: '10000',
      SYMBOL: 'TSLA',
      GRPC_HOST: 'http://45.77.223.21',
      PLAYGROUND_ENV: 'simulator',
      OPEN_STRATEGY: 'simple_open_strategy_v1',
      START_DATE: '2024-12-15',
      STOP_DATE: '2025-01-24',
      MODEL_UPDATE_FREQUENCY: 'weekly',
    },
    env_production : {
      ENV: 'production'
    },
    interpreter: 'src/cmd/stats/env/bin/python',
  },{
    name: 'supertrend1-meta',
    cmd: 'src/cmd/stats/trading_engine_optimizer.py',
    args: '',
    autorestart: false,
    watch: false,
    instances: 1,
    max_memory_restart: '1G',
    env_dev: {
      BALANCE: '10000',
      SYMBOL: 'META',
      GRPC_HOST: 'http://45.77.223.21',
      PLAYGROUND_ENV: 'simulator',
      OPEN_STRATEGY: 'simple_open_strategy_v1',
      START_DATE: '2024-12-15',
      STOP_DATE: '2025-01-24',
      MODEL_UPDATE_FREQUENCY: 'weekly',
    },
    env_production : {
      ENV: 'production'
    },
    interpreter: 'src/cmd/stats/env/bin/python',
  },{
    name: 'supertrend1-nvda',
    cmd: 'src/cmd/stats/trading_engine_optimizer.py',
    args: '',
    autorestart: false,
    watch: false,
    instances: 1,
    max_memory_restart: '1G',
    env_dev: {
      BALANCE: '10000',
      SYMBOL: 'NVDA',
      GRPC_HOST: 'http://45.77.223.21',
      PLAYGROUND_ENV: 'simulator',
      OPEN_STRATEGY: 'simple_open_strategy_v1',
      START_DATE: '2024-12-15',
      STOP_DATE: '2025-01-24',
      MODEL_UPDATE_FREQUENCY: 'weekly',
    },
    env_production : {
      ENV: 'production'
    },
    interpreter: 'src/cmd/stats/env/bin/python',
  }]
};