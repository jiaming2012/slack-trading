package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func PlaceOrder(ctx context.Context, url, token string, req *models.PlaceEquityTradeRequest) (map[string]interface{}, error) {
	if req.Quantity <= 0 {
		return nil, fmt.Errorf("placeTrade: quantity must be positive")
	}

	quantityStr := strconv.Itoa(req.Quantity)

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("PlaceTrade: failed to create request: %w", err)
	}

	symbol := strings.ToUpper(req.Symbol)

	q := httpReq.URL.Query()
	q.Add("class", "equity")
	q.Add("type", string(req.OrderType))
	q.Add("duration", string(eventmodels.TradeDurationDay))
	q.Add("symbol", symbol)
	q.Add("quantity", quantityStr)
	q.Add("side", string(req.Side))

	if req.Tag != "" {
		q.Add("tag", req.Tag)
	}

	if req.DryRun {
		q.Add("preview", "true")
	}

	httpReq.URL.RawQuery = q.Encode()
	httpReq.Header.Add("Accept", "application/json")
	httpReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	log.Infof("PlaceTrade: placing trade: %v", httpReq.URL.String())

	res, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("PlaceTrade: failed to place trade: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PlaceTrade: failed to place trade, http code %v", res.Status)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("PlaceTrade: failed to decode response: %w", err)
	}

	if e, found := response["errors"]; found {
		return nil, fmt.Errorf("PlaceTrade: failed to place trade: %v", e)
	}

	log.Infof("PlaceTrade: placed trade: %v", response)

	return response, nil
}
