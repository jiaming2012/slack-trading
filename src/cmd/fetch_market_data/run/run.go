package run

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func ReverseCandles(candles []*eventmodels.CandleDTO) []*eventmodels.CandleDTO {
	for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
		candles[i], candles[j] = candles[j], candles[i]
	}
	return candles
}

func GetOrderDateRange(order *eventmodels.TradierOrder) (time.Time, time.Time) {
	panic("not implemented")
}

func GetCandleAtDate(at time.Time, candles []*eventmodels.CandleDTO) (*eventmodels.CandleDTO, error) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("GetCandleAtDate: failed to load location: %w", err)
	}

	// Convert the time to EST
	createdAtEst := at.In(loc)
	target := createdAtEst.Format("2006-01-02 15:04:00")
	for _, candle := range candles {
		if candle.Date == target {
			return candle, nil
		}
	}

	return nil, fmt.Errorf("GetCandleAtDate: no candle found at %v", at.Format("2006-01-02 15:04"))
}

func TransformDateTime(candles []*eventmodels.CandleDTO) error {
	for _, candle := range candles {
		// Parse the input string assuming it's in a specific format.
		parsedTime, err := time.Parse("2006-01-02 15:04:05", candle.Date)
		if err != nil {
			return fmt.Errorf("TransformDateTime: failed to parse time: %w", err)
		}

		candle.Date = parsedTime.Format("2006-01-02 15:04")
	}

	return nil
}

func FetchFinancialModelingPrepChart(symbol eventmodels.StockSymbol, timeframeStr string, fromDate time.Time, toDate time.Time) ([]*eventmodels.CandleDTO, error) {
	apiKey := os.Getenv("FINANCIAL_MODELING_PREP_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("missing FINANCIAL_MODELING_PREP_API_KEY environment variable")
	}

	// Parse the base URL
	parsedURL, err := url.Parse("https://financialmodelingprep.com/api/v3/historical-chart")
	if err != nil {
		return nil, fmt.Errorf("fetchStockTicks: failed to parse base URL: %w", err)
	}

	// Join the additional path
	joinedPath := path.Join(parsedURL.Path, timeframeStr, string(symbol))
	parsedURL.Path = joinedPath

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("fetchStockTicks: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("from", fromDate.Format("2006-01-02"))
	q.Add("to", toDate.Format("2006-01-02"))
	q.Add("apikey", apiKey)

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchStockTicks: failed to fetch stock tick: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchStockTicks: failed to fetch stock tick, http code %v", res.Status)
	}

	var dto []*eventmodels.CandleDTO
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("fetchOptionContractTicks: failed to decode json: %w", err)
	}

	if len(dto) == 0 {
		return nil, fmt.Errorf("fetchOptionContractTicks: no data returned")
	}

	return dto, nil
}
