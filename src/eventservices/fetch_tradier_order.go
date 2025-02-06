package eventservices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func FetchTradierOrder(baseUrl, bearerToken string, orderID int) (*eventmodels.TradierOrderSpreadDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	// Parse the base URL
	parsedUrl, err := url.Parse(baseUrl)
	if err != nil {
		return nil, fmt.Errorf("FetchTradierOrder: failed to parse base URL: %w", err)
	}

	// Properly append the orderID to the path
	parsedUrl.Path, err = url.JoinPath(parsedUrl.Path, fmt.Sprintf("%d", orderID))
	if err != nil {
		return nil, fmt.Errorf("FetchTradierOrder: failed to join path: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, parsedUrl.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("FetchTradierOrder: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("includeTags", "true")

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	log.Tracef("fetching from %v", req.URL.String())

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FetchTradierOrder: failed to fetch option chain: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FetchTradierOrder: failed to fetch option chain, http code %v, fetching from %v", res.Status, req.URL.String())
	}

	var dto eventmodels.TradierOrderSpreadDTO
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("FetchTradierOrder: failed to decode json: %w", err)
	}

	return &dto, nil
}
