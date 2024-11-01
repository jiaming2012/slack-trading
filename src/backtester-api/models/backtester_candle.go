package models

import "github.com/jiaming2012/slack-trading/src/eventmodels"

type BacktesterCandle struct {
	Symbol eventmodels.Instrument             `json:"symbol"`
	Bar    *eventmodels.PolygonAggregateBarV2 `json:"candle"`
}
