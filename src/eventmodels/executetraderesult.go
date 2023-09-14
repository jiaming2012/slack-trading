package eventmodels

import (
	"github.com/google/uuid"
	"slack-trading/src/models"
)

type ExecuteOpenTradeResult struct {
	RequestID uuid.UUID                      `json:"id"`
	Side      string                         `json:"side"`
	Result    *models.ExecuteOpenTradeResult `json:"result"`
}

func (r *ExecuteOpenTradeResult) GetRequestID() uuid.UUID {
	return r.RequestID
}

type ExecuteCloseTradesResult struct {
	RequestID uuid.UUID                      `json:"id"`
	Side      string                         `json:"side"`
	Result    *models.ExecuteOpenTradeResult `json:"result"`
}

func (r *ExecuteCloseTradesResult) GetRequestID() uuid.UUID {
	return r.RequestID
}
