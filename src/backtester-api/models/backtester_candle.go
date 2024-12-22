package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterCandle struct {
	Symbol eventmodels.Instrument             `json:"symbol"`
	Period time.Duration                      `json:"period"`
	Bar    *eventmodels.PolygonAggregateBarV2 `json:"candle"`
}
