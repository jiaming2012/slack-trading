package models

import (
	"encoding/json"
	"time"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

type CandleRepositoryDTO struct {
	Symbol                   string        `json:"symbol"`
	Duration                 time.Duration `json:"duration"`
	FetchInterval            string        `json:"fetchInterval"`
	PolygonTimespanMultipler int           `json:"polygonTimespanMultipler"`
	PolygonTimespanUnit      string        `json:"polygonTimespanUnit"`
	Indicators               []string      `json:"indicators"`
	Position                 int           `json:"position"`
	StartingPosition         *int          `json:"startingPosition"`
	SourceType               string        `json:"sourceType"`
	IsInitialTick            bool          `json:"isInitialTick"`
	HistoryInDays            uint32        `json:"historyInDays"`
}

// Use custom JSON marshalling to print the nil pointer
func (r *CandleRepositoryDTO) MarshalJSON() ([]byte, error) {
	type Alias CandleRepositoryDTO
	return json.Marshal(&struct {
		*Alias
		StartingPosition *int `json:"startingPosition"`
	}{
		Alias:            (*Alias)(r),
		StartingPosition: r.StartingPosition,
	})
}

func (r *CandleRepositoryDTO) ToCreateRepositoryRequest() (models.CreateRepositoryRequest, error) {
	return models.CreateRepositoryRequest{
		Symbol: r.Symbol,
		Timespan: models.PolygonTimespanRequest{
			Multiplier: r.PolygonTimespanMultipler,
			Unit:       r.PolygonTimespanUnit,
		},
		HistoryInDays: r.HistoryInDays,
		Source: models.RepositorySource{
			Type: models.RepositorySourceType(r.SourceType),
		},
		Indicators: r.Indicators,
	}, nil
}

// func (r *CandleRepositoryDTO) ToCandleRepository(candles []*models.PolygonAggregateBarV2, queue *models.FIFOQueue[*BacktesterCandle]) (*CandleRepository, error) {
// 	return NewCandleRepository(
// 		models.NewStockSymbol(r.Symbol),
// 		r.Duration,
// 		candles,
// 		r.Indicators,
// 		queue,
// 		r.HistoryInDays,
// 		models.CandleRepositorySource{
// 			Type: r.SourceType,
// 		},
// 	)
// }
