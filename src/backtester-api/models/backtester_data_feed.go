package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterDataFeed interface {
	GetSymbol() eventmodels.Instrument
	SetStartingPosition(currentTime time.Time)
	GetPeriod() time.Duration
	GetSource() string
	FetchCandles(startTime, endTime time.Time) ([]*eventmodels.AggregateBarWithIndicators, error)
}
