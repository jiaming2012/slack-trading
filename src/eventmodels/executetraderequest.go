package eventmodels

import (
	"github.com/google/uuid"
	"slack-trading/src/models"
)

type ExecuteCloseTradesRequest struct {
	RequestID          uuid.UUID
	CloseTradesRequest *models.CloseTradesRequestV2
}

type ExecuteOpenTradeRequest struct {
	RequestID        uuid.UUID
	OpenTradeRequest *models.OpenTradeRequest
}
