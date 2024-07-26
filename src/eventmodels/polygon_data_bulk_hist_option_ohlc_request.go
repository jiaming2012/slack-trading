package eventmodels

import "time"

type PolygonDataBulkHistOptionOHLCRequest struct {
	ExpirationLessThanEqual    time.Time
	ExpirationGreaterThanEqual time.Time
	Interval                   time.Duration
	// Right      eventmodels.OptionType
	Root      StockSymbol
	StartDate time.Time
	EndDate   time.Time
	Spread    float64
}
