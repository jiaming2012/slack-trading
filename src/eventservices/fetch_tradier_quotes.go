package eventservices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func FetchTradierQuotes(baseUrl, bearerToken string, symbol eventmodels.StockSymbol, date time.Time) (*eventmodels.TradierMarketsHistoryResponseDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, baseUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("FetchTradierOrder: failed to create request: %w", err)
	}

	dateString := date.Format("2006-01-02")

	q := req.URL.Query()
	q.Add("symbol", string(symbol))
	q.Add("start", dateString)
	q.Add("end", dateString)

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FetchTradierOrder: failed to fetch option chain: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FetchTradierOrder: failed to fetch option chain, http code %v", res.Status)
	}

	var dto eventmodels.TradierMarketsHistoryResponseDTO
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("FetchTradierOrder: failed to decode json: %w", err)
	}

	if dto.History.Day.Date != dateString {
		return nil, fmt.Errorf("FetchTradierOrder: expected date %s, got %s", dateString, dto.History.Day.Date)
	}

	if dto.History.Day.Open == 0 {
		return nil, fmt.Errorf("FetchTradierOrder: open price is 0")
	}

	if dto.History.Day.High == 0 {
		return nil, fmt.Errorf("FetchTradierOrder: high price is 0")
	}

	if dto.History.Day.Low == 0 {
		return nil, fmt.Errorf("FetchTradierOrder: low price is 0")
	}

	if dto.History.Day.Close == 0 {
		return nil, fmt.Errorf("FetchTradierOrder: close price is 0")
	}

	return &dto, nil
}
