package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
)

type BacktesterCandle struct {
	Symbol eventmodels.Instrument                  `json:"symbol"`
	Period time.Duration                           `json:"period"`
	Bar    *eventmodels.AggregateBarWithIndicators `json:"candle"`
}
