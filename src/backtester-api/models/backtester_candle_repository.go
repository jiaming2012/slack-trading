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

func (r *BacktesterCandleRepository) Update(currentTime time.Time) (*eventmodels.PolygonAggregateBarV2, error) {
	if r.position >= len(r.candles) {
		return nil, fmt.Errorf("no more candles")
	}

	var newCandle *eventmodels.PolygonAggregateBarV2
	for {
		if r.position >= len(r.candles)-1 {
			break
		}

		nextCandleTimestamp := r.candles[r.position+1].Timestamp
		if currentTime.Equal(nextCandleTimestamp) || currentTime.After(nextCandleTimestamp) {
			r.position++
			newCandle = r.GetCurrentCandle()
		} else {
			break
		}
	}

	return newCandle, nil
}

func NewBacktesterCandleRepository(candles []*eventmodels.PolygonAggregateBarV2) *BacktesterCandleRepository {
	return &BacktesterCandleRepository{
		candles:  candles,
		position: 0,
	}
}
