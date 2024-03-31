package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

type CloseTradeRequest struct {
	BaseRequestEvent2
	AccountName     string     `json:"AccountName"`
	StrategyName    string     `json:"strategyName"`
	PriceLevelIndex int        `json:"priceLevelIndex"`
	Timeframe       *int       `json:"timeframe"`
	Percent         float64    `json:"percent"`
	Reason          string     `json:"reason"`
	Error           chan error `json:"-"`
}

func (r *CloseTradeRequest) Wait() chan error {
	return r.Error
}

func (r *CloseTradeRequest) GetMetaData() *MetaData {
	return r.Meta
}

func (r *CloseTradeRequest) ParseHTTPRequest(req *http.Request) error {
	if err := json.NewDecoder(req.Body).Decode(&r); err != nil {
		return fmt.Errorf("CloseTradeRequest.ParseHTTPRequest: failed to decode json: %w", err)
	}

	return nil
}

func (r *CloseTradeRequest) Validate(request *http.Request) error {
	if len(r.AccountName) == 0 {
		return fmt.Errorf("validate: AccountName not set")
	}

	if len(r.StrategyName) == 0 {
		return fmt.Errorf("validate: strategyName not set")
	}

	if r.PriceLevelIndex < 0 {
		return fmt.Errorf("validate: price level must be >= 0")
	}

	if r.Timeframe != nil && *r.Timeframe <= 0 {
		return fmt.Errorf("validate: timeframe must be > 0")
	}

	if r.Percent < 0 || r.Percent > 1 {
		return fmt.Errorf("validate: percent must be between 0 and 1")
	}

	if r.Reason == "" {
		return fmt.Errorf("validate: reason must be set")
	}

	return nil
}

func NewCloseTradeRequest(requestID uuid.UUID, accountName string, strategyName string, priceLevelIndex int, timeframe *int, percent float64, reason string) (*CloseTradeRequest, error) {
	req := &CloseTradeRequest{AccountName: accountName, StrategyName: strategyName, PriceLevelIndex: priceLevelIndex, Timeframe: timeframe, Percent: percent, Reason: reason}

	req.SetMetaData(&MetaData{RequestID: requestID})

	if err := req.Validate(nil); err != nil {
		return nil, err
	}

	return req, nil
}
