package models

import "github.com/jiaming2012/slack-trading/src/eventmodels"

type BacktesterCandle struct {
	Symbol eventmodels.Instrument             `json:"symbol"`
	Candle *eventmodels.PolygonAggregateBarV2 `json:"candle"`
}
