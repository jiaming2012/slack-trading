package eventservices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func makeRequestURL(symbol eventmodels.StockSymbol, timeframeValue int, timeframeUnit string, fromDate time.Time, toDate time.Time, apiKey string) (string, error) {
	// Parse the base URL
	parsedURL, err := url.Parse("https://api.polygon.io/v2/aggs/ticker")
	if err != nil {
		return "", fmt.Errorf("FetchPolygonStockChart: failed to parse base URL: %w", err)
	}

	// Join the additional path
	joinedPath := path.Join(parsedURL.Path, string(symbol), "range", fmt.Sprintf("%d", timeframeValue), timeframeUnit, fromDate.Format("2006-01-02"), toDate.Format("2006-01-02"))
	parsedURL.Path = joinedPath

	return parsedURL.String(), nil
}

func fetchPolygonStockChart(url, apiKey string) (*eventmodels.PolygonCandleResponse, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchPolygonStockChart: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("sort", "asc")
	q.Add("adjusted", "false")
	q.Add("apiKey", apiKey)

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")

	log.Infof("fetching from %v", req.URL.String())

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchPolygonStockChart: failed to fetch stock tick: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchPolygonStockChart: failed to fetch stock tick, http code %v", res.Status)
	}

	var dto eventmodels.PolygonCandleResponse
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("fetchPolygonStockChart: failed to decode json: %w", err)
	}

	if dto.NextURL != nil {
		log.Warnf("fetchPolygonStockChart: next url: %v", *dto.NextURL)
	}

	return &dto, nil
}

func FetchPolygonIndexChart(symbol eventmodels.StockSymbol, timeframeValue int, timeframeUnit string, fromDate time.Time, toDate time.Time) (*eventmodels.PolygonCandleResponse, error) {
	symbol = eventmodels.StockSymbol(fmt.Sprintf("I:%v", symbol))
	return FetchPolygonStockChart(symbol, timeframeValue, timeframeUnit, fromDate, toDate)
}

func FetchPolygonStockChart(symbol eventmodels.StockSymbol, timeframeValue int, timeframeUnit string, fromDate time.Time, toDate time.Time) (*eventmodels.PolygonCandleResponse, error) {
	apiKey := os.Getenv("POLYGON_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("missing POLYGON_API_KEY environment")
	}

	url, err := makeRequestURL(symbol, timeframeValue, timeframeUnit, fromDate, toDate, apiKey)
	if err != nil {
		return nil, fmt.Errorf("FetchPolygonStockChart: failed to make request URL: %w", err)
	}

	var aggregateResult eventmodels.PolygonCandleResponse

	for {
		resp, err := fetchPolygonStockChart(url, apiKey)
		if err != nil {
			return nil, fmt.Errorf("FetchPolygonStockChart: failed to fetch stock chart: %w", err)
		}

		aggregateResult.Ticker = resp.Ticker
		aggregateResult.QueryCount += resp.QueryCount
		aggregateResult.ResultsCount += resp.ResultsCount
		aggregateResult.Adjusted = resp.Adjusted

		aggregateResult.Results = append(aggregateResult.Results, resp.Results...)

		if resp.NextURL == nil {
			break
		}

		url = *resp.NextURL
	}

	if len(aggregateResult.Results) == 0 {
		return nil, fmt.Errorf("FetchPolygonStockChart: no results found")
	}

	return &aggregateResult, nil
}

func FetchPolygonAggregateBars() eventmodels.FetchDataFunc[eventmodels.PolygonAggregateBar] {
	return func(url, apiKey string) (*eventmodels.AggregateResult[eventmodels.PolygonAggregateBar], error) {
		client := http.Client{
			Timeout: 10 * time.Second,
		}

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("FetchPolygonAggregateBars: failed to create request: %w", err)
		}

		req.Header.Add("Accept", "application/json")

		q := req.URL.Query()
		q.Add("adjusted", "false")
		q.Add("limit", "50000")
		q.Add("sort", "asc")
		q.Add("apiKey", apiKey)

		req.URL.RawQuery = q.Encode()

		log.Infof("FetchPolygonAggregateBars: fetching option contracts from %v", req.URL.String())

		res, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("FetchPolygonAggregateBars: failed to fetch option contracts: %w", err)
		}

		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("FetchPolygonAggregateBars: failed to fetch option contracts, http code %v", res.Status)
		}

		var dto eventmodels.PolygonGetV3ReferenceOptionsContractsResponse[eventmodels.PolygonAggregateBar]
		if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
			return nil, fmt.Errorf("FetchPolygonAggregateBars: failed to decode json: %w", err)
		}

		return &eventmodels.AggregateResult[eventmodels.PolygonAggregateBar]{
			QueryCount:   1,
			ResultsCount: len(dto.Results),
			Results:      dto.Results,
			GetNextURL:   func() *string { return dto.NextURL },
		}, nil
	}
}
