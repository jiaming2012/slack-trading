package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

type BacktesterCandle struct {
	Symbol models.Instrument                  `json:"symbol"`
	Period time.Duration                           `json:"period"`
	Bar    *models.AggregateBarWithIndicators `json:"candle"`
}
