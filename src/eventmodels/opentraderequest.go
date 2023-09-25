package eventmodels

import (
	"fmt"
	"github.com/google/uuid"
)

type OpenTradeRequest struct {
	RequestID    uuid.UUID
	AccountName  string `json:"AccountName"`
	StrategyName string `json:"strategyName"`
	Timeframe    *int   `json:"timeframe"`
}

func NewOpenTradeRequest(requestID uuid.UUID, accountName string, strategyName string, timeframe *int) (*OpenTradeRequest, error) {
	req := &OpenTradeRequest{RequestID: requestID, AccountName: accountName, StrategyName: strategyName, Timeframe: timeframe}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	return req, nil
}

func (r *OpenTradeRequest) Validate() error {
	if len(r.AccountName) == 0 {
		return fmt.Errorf("validate: AccountName not set")
	}

	if len(r.StrategyName) == 0 {
		return fmt.Errorf("validate: strategyName not set")
	}

	if r.Timeframe != nil && *r.Timeframe <= 0 {
		return fmt.Errorf("validate: timeframe must be greater than zero")
	}

	return nil
}
