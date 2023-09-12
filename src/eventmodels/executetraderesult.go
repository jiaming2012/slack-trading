package eventmodels

import (
	"github.com/google/uuid"
	"slack-trading/src/models"
	"time"
)

type ExecuteOpenTradeResult struct {
	Id        uuid.UUID                      `json:"id"`
	Symbol    string                         `json:"symbol"`
	Side      string                         `json:"side"`
	Timestamp time.Time                      `json:"timestamp"`
	Result    *models.ExecuteOpenTradeResult `json:"result"`
}
