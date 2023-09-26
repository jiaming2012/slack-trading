package eventmodels

import (
	"github.com/google/uuid"
	"slack-trading/src/models"
)

type FetchTradesRequest struct {
	RequestID    uuid.UUID `json:"requestID"`
	AccountName  string    `json:"accountName"`
	StrategyName *string   `json:"strategyName"`
}

func (r *FetchTradesRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}

func NewFetchTradesRequest(requestID uuid.UUID, accountName string, strategyName *string) *FetchTradesRequest {
	return &FetchTradesRequest{RequestID: requestID, AccountName: accountName, StrategyName: strategyName}
}

type FetchTradesResult struct {
	RequestID uuid.UUID             `json:"requestID"`
	Trades    []*models.TradeLevels `json:"trades"`
}

func (r *FetchTradesResult) GetRequestID() uuid.UUID {
	return r.RequestID
}

func NewFetchTradesResult(requestID uuid.UUID, trades []*models.TradeLevels) *FetchTradesResult {
	return &FetchTradesResult{RequestID: requestID, Trades: trades}
}
