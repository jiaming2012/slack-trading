package eventservices

import (
	"fmt"
	"net/http"
	"time"

	polygon "github.com/polygon-io/client-go/rest"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type PolygonTickDataMachine struct {
	Client *polygon.Client
	ApiKey string
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

	if from == nil {
		return nil, fmt.Errorf("from date is nil")
	}

	if to == nil {
		return nil, fmt.Errorf("to date is nil")
	}

	// start at stock market open
	fromDate := time.Date(from.Year, time.Month(from.Month), from.Day, 9, 30, 0, 0, loc)

	// end at stock market close
	toDate := time.Date(to.Year, time.Month(to.Month), to.Day, 16, 0, 0, 0, loc)

	return m.FetchAggregateBarsWithDates(ticker, timespan, fromDate, toDate, loc)
}

func (m *PolygonTickDataMachine) FetchPastCandles(symbol eventmodels.StockSymbol, timespan eventmodels.PolygonTimespan, daysPast int, end *eventmodels.PolygonDate) ([]*eventmodels.PolygonAggregateBarV2, error) {
	to := end.GetPreviousDay(1)
	from := to.GetPreviousDay(daysPast)
	maxAttempts := 5

	errMsg := ""
	for i := 0; true; i++ {
		pastBars, err := m.FetchAggregateBars(eventmodels.StockSymbol(symbol), timespan, from, to)
		if err != nil {
			if i == maxAttempts-1 {
				errMsg = fmt.Sprintf("failed to fetch past candles from %s to %s: %v", from.ToString(), to.ToString(), err)
				break
			}

			from = from.GetPreviousDay(1)
			time.Sleep(10 * time.Millisecond)

			continue
		}

		return pastBars, nil
	}

	return nil, eventmodels.NewWebError(500, errMsg, nil)
}

func (m *PolygonTickDataMachine) FetchAggregateBarsWithDates(ticker eventmodels.Instrument, timespan eventmodels.PolygonTimespan, fromDate, toDate time.Time, loc *time.Location) ([]*eventmodels.PolygonAggregateBarV2, error) {
	var bars []*eventmodels.PolygonAggregateBarV2

	symbol := eventmodels.StockSymbol(ticker.GetTicker())
	result, err := FetchPolygonStockChart(symbol, timespan.Multiplier, string(timespan.Unit), fromDate, toDate, m.ApiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data from polygon api: %w", err)
	}

	for _, result := range result.Results {
		bar, err := result.ToCandle()
		if err != nil {
			return nil, fmt.Errorf("failed to convert result to candle dto: %w", err)
		}

		if isInBetween(bar.Timestamp, fromDate, toDate) {
			bars = append(bars, &eventmodels.PolygonAggregateBarV2{
				Volume:    bar.Volume,
				Open:      bar.Open,
				Close:     bar.Close,
				High:      bar.High,
				Low:       bar.Low,
				Timestamp: bar.Timestamp,
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

func NewPolygonClient(apiKey string) *PolygonTickDataMachine {
	return &PolygonTickDataMachine{
		Client: polygon.New(apiKey),
		ApiKey: apiKey,
	}
}
