package router

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/models"
)

func (s Server) fetchOptionCandles(playgroundID uuid.UUID, symbol models.OptionSymbol, period time.Duration, from time.Time, to *time.Time) ([]*models.AggregateBarWithIndicators, error) {
	result, err := s.optionsClient.FetchPolygonOptionAggregateBars(playgroundID, symbol, period, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetchOptionCandles: failed to fetch option candles: %w", err)
	}

	var candles []*models.AggregateBarWithIndicators
	for _, bar := range result.Results {
		timestamp := time.Unix(int64(bar.Time), 0)
		candles = append(candles, &models.AggregateBarWithIndicators{
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

func (s Server) fetchCandles(playgroundID uuid.UUID, symbol models.StockSymbol, period time.Duration, from time.Time, to *time.Time) ([]*models.AggregateBarWithIndicators, error) {
	playground, err := s.dbService.GetPlayground(playgroundID)
	if err != nil {
		return nil, models.NewWebError(404, "handleCandles: playground not found", nil)
	}

	candles, err := playground.FetchCandles(symbol, period, from, to)
	if err != nil {
		return nil, models.NewWebError(500, "failed to fetch candles", err)
	}

	return candles, nil
}

func (s Server) nextTick(playgroundID uuid.UUID, duration time.Duration, isPreview bool) (*backtester_models.TickDelta, error) {
	playground, err := s.dbService.GetPlayground(playgroundID)
	if err != nil {
		return nil, fmt.Errorf("playground not found")
	}

	tickDelta, err := playground.Tick(duration, isPreview, s.dbService)
	if err != nil {
		return nil, fmt.Errorf("failed to tick: %v", err)
	}

	// Embed account state so clients can skip the separate GetAccount RPC.
	positionCache, err := playground.UpdatePricesAndGetPositionCache()
	if err != nil {
		// Non-fatal: log and return tick without account state.
		log.Warnf("nextTick: failed to get position cache: %v", err)
		return tickDelta, nil
	}

	tickDelta.Balance = playground.GetBalance()
	tickDelta.Equity = playground.GetEquity(positionCache)
	tickDelta.FreeMargin = playground.GetFreeMarginFromPositionMap(positionCache)
	tickDelta.Positions = positionCache.Iter()

	return tickDelta, nil
}
