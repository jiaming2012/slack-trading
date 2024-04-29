package eventservices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"slack-trading/src/eventmodels"
)

func FetchOptionContractTicks(url, bearerToken string, symbol eventmodels.StockSymbol, expiration string) ([]*eventmodels.OptionChainTickDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("FetchOptionContractTicks: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("symbol", string(symbol))
	q.Add("expiration", expiration)

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FetchOptionContractTicks: failed to fetch option chain: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FetchOptionContractTicks: failed to fetch option chain, http code %v", res.Status)
	}

	var dto eventmodels.OptionContractChainDTO
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("FetchOptionContractTicks: failed to decode json: %w", err)
	}

	return dto.Options.Values, nil
}
