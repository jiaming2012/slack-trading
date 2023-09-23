package eventmodels

import (
	"github.com/google/uuid"
	"slack-trading/src/models"
)

type ExecuteCloseTradesRequest struct {
	RequestID          uuid.UUID
	CloseTradesRequest *models.CloseTradesRequest
}

type ExecuteOpenTradeRequest struct {
	RequestID        uuid.UUID
	OpenTradeRequest *models.OpenTradeRequest
}
