package eventmodels

import (
	"github.com/google/uuid"
	"slack-trading/src/models"
)

type ExecuteCloseTradeRequest struct {
	PriceLevel         *models.PriceLevel
	CloseTradesRequest models.CloseTradesRequest
}

type ExecuteOpenTradeRequest struct {
	RequestID        uuid.UUID
	OpenTradeRequest *models.OpenTradeRequest
}
