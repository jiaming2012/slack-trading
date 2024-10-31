# Backtestester API
Provides a unified model to **train models** -> **backtest models** -> **live trading**

`Playground.Tick()` will automatically increment to the next tick before applying any processing. Hence in the following example, the zero value of the tick data feed will be ignored, and the first tick processed will be `100.0`:

``` go
prices := []float64{0, 100.0, 115.0}
feed := mock.NewMockBacktesterDataFeed()
```

## Protobufs
Protobufs are used to speed up communication with API clients.

## Compiling
``` bash
cd ${PROJECTS_DIR}/src/backtester-api
protoc --go_out=./proto --go_opt=paths=source_relative --go-grpc_out=./proto --go-grpc_opt=paths=source_relative playground.proto
```