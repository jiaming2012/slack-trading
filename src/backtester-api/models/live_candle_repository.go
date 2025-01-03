package models

import "github.com/jiaming2012/slack-trading/src/eventmodels"

type LiveCandleRepository struct {
	Instrument eventmodels.Instrument
	Period     eventmodels.TradierInterval
}
