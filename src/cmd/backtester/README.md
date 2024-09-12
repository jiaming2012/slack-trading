
## Usage
``` bash
cd path/to/src/cmd/backtester
go run src/main.go --outDir /Users/jamal/projects/grodt --symbol SPX
```

The program expects there to be a eventstream backtest-signals-SPX

## Creating backtest-signals
The backtester feeds off signals from EventStremDB.

Current method involves exporting TradingView data to `src/cmd/import_signals/csv_data`, or reprocessing files saved in `src/cmd/import_signals/processed`.

TODO: create signals directly from API stream.

#### Usage
Execute the following commands to import signal data
``` bash
cd path/to/src/cmd/import_signals
./run-dev.sh
```

## Add candle data
The backtester needs candlestick data to match the timeframe of the underlying strategy. For example, when processing `backtest-signals-SPX`, the backtester also requires `candles-SPX-15` and `candles-SPX-60`.

#### Usage
Execute the following commands to import candle data
``` bash
cd path/to/src/cmd/import_trading_view_data
./run-dev.sh
```

The backtester will end once all signals in the `backtest-signals` stream are processed.