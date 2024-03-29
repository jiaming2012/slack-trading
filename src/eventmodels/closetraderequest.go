package eventmodels

import (
	"fmt"
	"github.com/google/uuid"
)

type CloseTradeRequest struct {
	RequestID       uuid.UUID
	AccountName     string  `json:"AccountName"`
	StrategyName    string  `json:"strategyName"`
	PriceLevelIndex int     `json:"priceLevelIndex"`
	Timeframe       *int    `json:"timeframe"`
	Percent         float64 `json:"percent"`
	Reason          string  `json:"reason"`
}

func NewCloseTradeRequest(requestID uuid.UUID, accountName string, strategyName string, priceLevelIndex int, timeframe *int, percent float64, reason string) (*CloseTradeRequest, error) {
	req := &CloseTradeRequest{RequestID: requestID, AccountName: accountName, StrategyName: strategyName, PriceLevelIndex: priceLevelIndex, Timeframe: timeframe, Percent: percent, Reason: reason}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	return req, nil
}

func (r *CloseTradeRequest) Validate() error {
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
