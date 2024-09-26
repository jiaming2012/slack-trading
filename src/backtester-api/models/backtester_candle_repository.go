package models

import (
	"fmt"
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterCandleRepository struct {
	candles  []*eventmodels.PolygonAggregateBarV2
	position int
}

func (r *BacktesterCandleRepository) GetCurrentCandle() *eventmodels.PolygonAggregateBarV2 {
	if r.position >= len(r.candles) {
		return nil
	}

	return r.candles[r.position]
}

func (r *BacktesterCandleRepository) Next(currentTime time.Time) error {
	if r.position >= len(r.candles) {
		return fmt.Errorf("no more candles")
	}

	for r.position < len(r.candles) && currentTime.After(r.candles[r.position].Timestamp) {
		r.position++
	}

	return nil
}

func NewBacktesterCandleRepository(candles []*eventmodels.PolygonAggregateBarV2) *BacktesterCandleRepository {
	return &BacktesterCandleRepository{
		candles:  candles,
		position: 0,
	}
}
