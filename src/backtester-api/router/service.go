package router

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/data"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func (s Server) fetchCandles(playgroundID uuid.UUID, symbol eventmodels.StockSymbol, period time.Duration, from, to time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
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

	tickDelta, err := playground.Tick(duration, isPreview)
	if err != nil {
		return nil, fmt.Errorf("failed to tick: %v", err)
	}

	if playground.GetMeta().Environment == models.PlaygroundEnvironmentLive {
		if err := data.SaveEquityPlotRecord(playgroundID, tickDelta.EquityPlot.Timestamp, tickDelta.EquityPlot.Value); err != nil {
			return nil, fmt.Errorf("failed to save equity plot record: %v", err)
		}
	}

	return tickDelta, nil
}
