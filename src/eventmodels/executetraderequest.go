package eventmodels

import (
	"github.com/google/uuid"
	"slack-trading/src/models"
)

type ExecuteCloseTradeRequest struct {
	RequestID uuid.UUID
	Timeframe *int
	Trade     *models.Trade
	Percent   float64
}

type ExecuteCloseTradesRequest struct {
	RequestID          uuid.UUID
	CloseTradesRequest *models.CloseTradesRequest
}

type ExecuteOpenTradeRequest struct {
	RequestID        uuid.UUID
	OpenTradeRequest *models.OpenTradeRequest
}
