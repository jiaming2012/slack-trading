package eventmodels

import (
	"github.com/google/uuid"
	"slack-trading/src/models"
	"time"
)

type ExecuteOpenTradeResult struct {
	RequestID uuid.UUID                      `json:"id"`
	Symbol    string                         `json:"symbol"`
	Side      string                         `json:"side"`
	Timestamp time.Time                      `json:"timestamp"`
	Result    *models.ExecuteOpenTradeResult `json:"result"`
}

func (r *ExecuteOpenTradeResult) GetRequestID() uuid.UUID {
	return r.RequestID
}
