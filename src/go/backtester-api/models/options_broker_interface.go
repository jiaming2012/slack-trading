package models

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
)

type IOptionsBroker interface {
	GetCandles(playgroundID uuid.UUID, symbol eventmodels.OptionSymbol, period time.Duration, from time.Time, to *time.Time) ([]*eventmodels.AggregateBarWithIndicators, error)
	ExerciseOption(ctx context.Context, req *eventmodels.ExerciseOptionRequest) error
}
