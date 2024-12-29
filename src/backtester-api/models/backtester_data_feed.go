package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterDataFeed interface {
	GetSymbol() eventmodels.Instrument
	GetPeriod() time.Duration
	FetchCandles(startTime, endTime time.Time) ([]*eventmodels.AggregateBarWithIndicators, error)
}
