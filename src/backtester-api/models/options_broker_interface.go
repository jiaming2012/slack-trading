package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type IOptionsBroker interface {
	GetCandles(playgroundID uuid.UUID, symbol eventmodels.OptionSymbol, period time.Duration, from time.Time, to *time.Time) ([]*eventmodels.AggregateBarWithIndicators, error)
}
