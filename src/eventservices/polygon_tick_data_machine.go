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

func (m *PolygonTickDataMachine) FetchAggregateBarsDTO(ticker eventmodels.Instrument, timespan eventmodels.PolygonTimespan, from, to *eventmodels.PolygonDate) ([]*eventmodels.PolygonAggregateBarV2DTO, error) {
	bars, err := m.FetchAggregateBars(ticker, timespan, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch aggregate bars: %w", err)
	}

	var barsDTO []*eventmodels.PolygonAggregateBarV2DTO
	for _, bar := range bars {
		dto := bar.ToDTO()
		barsDTO = append(barsDTO, &dto)
	}

	return barsDTO, nil
}

func isInBetween(t time.Time, from, to time.Time) bool {
	return (t.Equal(from) || t.After(from)) && (t.Equal(to) || t.Before(to))
}

func (m *PolygonTickDataMachine) FetchAggregateBars(ticker eventmodels.Instrument, timespan eventmodels.PolygonTimespan, from, to *eventmodels.PolygonDate) ([]*eventmodels.PolygonAggregateBarV2, error) {
	// Load the location for New York (Eastern Time)
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("failed to load location America/New_York: %w", err)
	}

	// start at stock market open
	fromDate := time.Date(from.Year, time.Month(from.Month), from.Day, 9, 30, 0, 0, loc)

	// end at stock market close
	toDate := time.Date(to.Year, time.Month(to.Month), to.Day, 16, 0, 0, 0, loc)

	// fetch data from polygon api
	params := models.ListAggsParams{
		Ticker:     ticker.GetTicker(),
		Multiplier: timespan.Multiplier,
		Timespan:   models.Timespan(timespan.Unit),
		From:       models.Millis(fromDate),
		To:         models.Millis(toDate),
	}.WithOrder(models.Asc).WithAdjusted(false)

	// make request
	iter := m.Client.ListAggs(context.Background(), params)

	if iter.Err() != nil {
		return nil, fmt.Errorf("failed to fetch data from polygon api: %w", iter.Err())
	}

	// iterate over the results
	var bars []*eventmodels.PolygonAggregateBarV2

	for iter.Next() {
		tstamp := time.Time(iter.Item().Timestamp).In(loc)

		if isInBetween(tstamp, fromDate, toDate) {
			bars = append(bars, &eventmodels.PolygonAggregateBarV2{
				Volume:    iter.Item().Volume,
				VWAP:      iter.Item().VWAP,
				Open:      iter.Item().Open,
				Close:     iter.Item().Close,
				High:      iter.Item().High,
				Low:       iter.Item().Low,
				Timestamp: tstamp,
			})
		}
	}

	if len(bars) > 0 {
		// assert first bar is after the from date
		if !(bars[0].Timestamp.Equal(fromDate) || bars[0].Timestamp.After(fromDate)) {
			return nil, fmt.Errorf("first bar %v timestamp does not match from date %v", bars[0].Timestamp, fromDate)
		}

		// assert last bar is the to date
		if !(bars[len(bars)-1].Timestamp.Before(toDate) || bars[len(bars)-1].Timestamp.Equal(toDate)) {
			return nil, fmt.Errorf("last bar %v timestamp does not match to date %v", bars[len(bars)-1].Timestamp, toDate)
		}
	} else {
		return nil, fmt.Errorf("no bars returned")
	}

	return bars, nil
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

	timespan := eventmodels.PolygonTimespan{
		Multiplier: req.Multiplier,
		Unit:       eventmodels.PolygonTimespanUnit(req.Timespan),
	}

	from, err := eventmodels.NewPolygonDate(req.From)
	if err != nil {
		errCh <- fmt.Errorf("failed to parse from date: %w", err)
		return
	}

	to, err := eventmodels.NewPolygonDate(req.To)
	if err != nil {
		errCh <- fmt.Errorf("failed to parse to date: %w", err)
		return
	}

	bars, err := m.FetchAggregateBars(req.Symbol, timespan, from, to)
	if err != nil {
		errCh <- fmt.Errorf("failed to fetch aggregate bars: %w", err)
		return
	}

	resultCh <- bars
}

func NewPolygonTickDataMachine(apiKey string) *PolygonTickDataMachine {
	return &PolygonTickDataMachine{
		Client: polygon.New(apiKey),
	}
}
