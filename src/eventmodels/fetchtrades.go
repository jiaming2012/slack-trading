package eventmodels

import (
	"github.com/google/uuid"
	"slack-trading/src/models"
)

type FetchTradesRequest struct {
	RequestID    uuid.UUID
	AccountName  string
	StrategyName *string
}

func NewFetchTradesRequest(requestID uuid.UUID, accountName string, strategyName *string) *FetchTradesRequest {
	return &FetchTradesRequest{RequestID: requestID, AccountName: accountName, StrategyName: strategyName}
}

type FetchTradesResult struct {
	RequestID uuid.UUID
	Trades    []*models.PriceLevelTrades
}

func (r *FetchTradesResult) GetRequestID() uuid.UUID {
	return r.RequestID
}

func NewFetchTradesResult(requestID uuid.UUID, trades []*models.PriceLevelTrades) *FetchTradesResult {
	return &FetchTradesResult{RequestID: requestID, Trades: trades}
}
