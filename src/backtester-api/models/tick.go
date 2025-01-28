package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type Tick struct {
	Symbol    eventmodels.Instrument
	Timestamp time.Time
	Value     float64
}
