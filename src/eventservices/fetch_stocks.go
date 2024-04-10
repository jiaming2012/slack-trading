package eventservices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"slack-trading/src/eventmodels"
)

func FetchStockTicks(symbol, url, bearerToken string) (*eventmodels.StockTickItemDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchStockTicks: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("symbols", symbol)

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchStockTicks: failed to fetch stock tick: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchStockTicks: failed to fetch stock tick, http code %v", res.Status)
	}

	var dto eventmodels.StockTickDTO
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("fetchOptionContractTicks: failed to decode json: %w", err)
	}

	return &dto.Quotes.Tick, nil
}
