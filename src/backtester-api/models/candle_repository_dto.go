package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type CandleRepositoryDTO struct {
	Symbol                   string        `json:"symbol"`
	Duration                 time.Duration `json:"duration"`
	FetchInterval            string        `json:"fetchInterval"`
	PolygonTimespanMultipler int           `json:"polygonTimespanMultipler"`
	PolygonTimespanUnit      string        `json:"polygonTimespanUnit"`
	Indicators               []string      `json:"indicators"`
	Position                 int           `json:"position"`
	StartingPosition         int           `json:"startingPosition"`
	SourceType               string        `json:"sourceType"`
	IsInitialTick            bool          `json:"isInitialTick"`
	HistoryInDays            uint32        `json:"historyInDays"`
}

func (r *CandleRepositoryDTO) ToCreateRepositoryRequest() (eventmodels.CreateRepositoryRequest, error) {
	return eventmodels.CreateRepositoryRequest{
		Symbol: r.Symbol,
		Timespan: eventmodels.PolygonTimespanRequest{
			Multiplier: r.PolygonTimespanMultipler,
			Unit:       r.PolygonTimespanUnit,
		},
		HistoryInDays: r.HistoryInDays,
		Source: eventmodels.RepositorySource{
			Type: eventmodels.RepositorySourceType(r.SourceType),
		},
		Indicators: r.Indicators,
	}, nil
}

func (r *CandleRepositoryDTO) ToCandleRepository(candles []*eventmodels.PolygonAggregateBarV2, queue *eventmodels.FIFOQueue[*BacktesterCandle]) (*CandleRepository, error) {
	return NewCandleRepository(
		eventmodels.NewStockSymbol(r.Symbol),
		r.Duration,
		candles,
		r.Indicators,
		queue,
		r.StartingPosition,
		r.HistoryInDays,
		eventmodels.CandleRepositorySource{
			Type: r.SourceType,
		},
	)
}
