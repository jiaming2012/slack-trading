package eventmodels

import "time"

type FetchEVSpreadsRequest struct {
	StartsAt   time.Time
	EndsAt     time.Time
	Ticker     StockSymbol
	GoEnv      string
	SignalName SignalName
}
