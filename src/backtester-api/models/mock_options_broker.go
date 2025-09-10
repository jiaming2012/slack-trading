package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type MockOptionsBroker struct {
	data map[eventmodels.OptionSymbol][]*eventmodels.AggregateBarWithIndicators
}

func (b *MockOptionsBroker) GetCandles(playgroundID uuid.UUID, symbol eventmodels.OptionSymbol, period time.Duration, from time.Time, to *time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
	data, ok := b.data[symbol]
	if !ok {
		return nil, fmt.Errorf("failed to find option symbol for %v", symbol)
	}

	var results []*eventmodels.AggregateBarWithIndicators
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
