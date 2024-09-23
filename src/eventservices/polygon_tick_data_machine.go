package eventservices

import (
	"context"
	"fmt"
	"net/http"
	"time"

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/models"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type PolygonTickDataMachine struct {
	Client *polygon.Client
}

func (m *PolygonTickDataMachine) Serve(r *http.Request, apiRequest eventmodels.ApiRequest3, resultCh chan interface{}, errCh chan error) {
	dto, ok := apiRequest.(*eventmodels.PolygonDataReadRequestDTO)
	if !ok {
		errCh <- eventmodels.ErrInvalidRequestType
		return
	}

	req, err := dto.ToModel()
	if err != nil {
		errCh <- fmt.Errorf("failed to convert dto to model: %w", err)
		return
	}

	log.Debugf("fetching polygon tick data from api for symbol %s", req.Symbol)

	// fetch data from polygon api
	params := models.ListAggsParams{
		Ticker:     req.Symbol.String(),
		Multiplier: req.Multiplier,
		Timespan:   req.Timespan,
		From:       models.Millis(req.From),
		To:         models.Millis(req.To),
	}.WithOrder(models.Asc).WithAdjusted(true)

	// make request
	iter := m.Client.ListAggs(context.Background(), params)

	// iterate over the results
	var bars []interface{}
	for iter.Next() {
		b := eventmodels.PolygonAggregateBarV2{
			Volume:    iter.Item().Volume,
			VWAP:      iter.Item().VWAP,
			Open:      iter.Item().Open,
			Close:     iter.Item().Close,
			High:      iter.Item().High,
			Low:       iter.Item().Low,
			Timestamp: time.Time(iter.Item().Timestamp),
		}

		bars = append(bars, b.ToDTO())
	}

	resultCh <- bars
}

func NewPolygonTickDataMachine(apiKey string) *PolygonTickDataMachine {
	return &PolygonTickDataMachine{
		Client: polygon.New(apiKey),
	}
}
