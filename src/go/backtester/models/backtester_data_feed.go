package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

type BacktesterDataFeed interface {
	GetSymbol() models.Instrument
	SetStartingPosition(currentTime time.Time)
	GetPeriod() time.Duration
	GetSource() string
	FetchCandles(startTime, endTime time.Time) ([]*models.AggregateBarWithIndicators, error)
}
