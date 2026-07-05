package models

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

type IOptionsBroker interface {
	GetCandles(playgroundID uuid.UUID, symbol models.OptionSymbol, period time.Duration, from time.Time, to *time.Time) ([]*models.AggregateBarWithIndicators, error)
	ExerciseOption(ctx context.Context, req *models.ExerciseOptionRequest) error
}
