package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

type MockOptionsBroker struct {
	data map[models.OptionSymbol][]*models.AggregateBarWithIndicators
}

func (b *MockOptionsBroker) ExerciseOption(ctx context.Context, req *models.ExerciseOptionRequest) error {
	return fmt.Errorf("not implemented")
}

func (b *MockOptionsBroker) GetCandles(playgroundID uuid.UUID, symbol models.OptionSymbol, period time.Duration, from time.Time, to *time.Time) ([]*models.AggregateBarWithIndicators, error) {
	data, ok := b.data[symbol]
	if !ok {
		return nil, fmt.Errorf("failed to find option symbol for %v", symbol)
	}

	var results []*models.AggregateBarWithIndicators
	for _, bar := range data {
		if !bar.Timestamp.Before(from) {
			if to != nil && bar.Timestamp.After(*to) {
				continue
			}

			results = append(results, bar)
		}
	}

	return results, nil
}
