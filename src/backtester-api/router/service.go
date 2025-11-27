package router

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func (s Server) fetchOptionCandles(playgroundID uuid.UUID, symbol eventmodels.OptionSymbol, period time.Duration, from time.Time, to *time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
	result, err := s.optionsClient.FetchPolygonOptionAggregateBars(playgroundID, symbol, period, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetchOptionCandles: failed to fetch option candles: %w", err)
	}

	var candles []*eventmodels.AggregateBarWithIndicators
	for _, bar := range result.Results {
		timestamp := time.Unix(int64(bar.Time), 0)
		candles = append(candles, &eventmodels.AggregateBarWithIndicators{
			Timestamp: timestamp,
			Open:      bar.Open,
			Close:     bar.Close,
			High:      bar.High,
			Low:       bar.Low,
		})
	}

	// todo: add a cache for the candles

	return candles, nil
}

func (s Server) fetchCandles(playgroundID uuid.UUID, symbol eventmodels.StockSymbol, period time.Duration, from time.Time, to *time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
	playground, err := s.dbService.GetPlayground(playgroundID)
	if err != nil {
		return nil, eventmodels.NewWebError(404, "handleCandles: playground not found", nil)
	}

	candles, err := playground.FetchCandles(symbol, period, from, to)
	if err != nil {
		return nil, eventmodels.NewWebError(500, "failed to fetch candles", err)
	}

	return candles, nil
}

func (s Server) nextTick(playgroundID uuid.UUID, duration time.Duration, isPreview bool) (*models.TickDelta, error) {
	playground, err := s.dbService.GetPlayground(playgroundID)
	if err != nil {
		return nil, fmt.Errorf("playground not found")
	}

	tickDelta, err := playground.Tick(duration, isPreview, s.dbService)
	if err != nil {
		return nil, fmt.Errorf("failed to tick: %v", err)
	}

	return tickDelta, nil
}
