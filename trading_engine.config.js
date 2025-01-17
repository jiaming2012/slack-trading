module.exports = {
  apps : [{
    name: 'supertrend-momentum-aapl',
    cmd: '/Users/jamal/projects/slack-trading/src/cmd/stats/trading_engine.py',
    args: '',
    autorestart: false,
    watch: true,
    instances: 1,
    max_memory_restart: '1G',
    env: {
      BALANCE: '10000',
      SYMBOL: 'AAPL',
      GRPC_HOST: 'http://localhost:5051',
      ENV: 'development'
    },
    env_production : {
      ENV: 'production'
    },
    interpreter: '/Users/jamal/projects/slack-trading/src/cmd/stats/env/bin/python',
  },{
    name: 'supertrend-momentum-coin',
    cmd: '/Users/jamal/projects/slack-trading/src/cmd/stats/trading_engine.py',
    args: '',
    autorestart: false,
    watch: true,
    instances: 1,
    max_memory_restart: '1G',
    env: {
      BALANCE: '10000',
      SYMBOL: 'COIN',
      GRPC_HOST: 'http://localhost:5051',
      ENV: 'development'
    },
    env_production : {
      ENV: 'production'
    },
    interpreter: '/Users/jamal/projects/slack-trading/src/cmd/stats/env/bin/python',
  }]
};