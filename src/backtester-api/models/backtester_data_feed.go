package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterDataFeed interface {
	GetSymbol() eventmodels.Instrument
	FetchRange(startTime, endTime time.Time) ([]*eventmodels.PolygonAggregateBarV2, error)
}
