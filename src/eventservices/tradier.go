package eventservices

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func FetchTradierPositions(url string, token string) ([]eventmodels.TradierPositionDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("FetchTradierPositions: failed to create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FetchTradierPositions: failed to fetch positions: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FetchTradierPositions: failed to fetch positions: %s", res.Status)
	}

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("FetchTradierPositions: failed to read response body: %w", err)
	}

	positions, err := utils.ParseTradierResponse[eventmodels.TradierPositionDTO](bytes)
	if err != nil {
		return nil, fmt.Errorf("FetchTradierPositions: failed to parse response: %w", err)
	}

	return positions, nil
}
