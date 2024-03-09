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

func (r ExecuteCloseTradeRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}

type ExecuteCloseTradesRequest struct {
	RequestID          uuid.UUID
	CloseTradesRequest *models.CloseTradesRequest
}

func (r ExecuteCloseTradesRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}

type ExecuteOpenTradeRequest struct {
	RequestID        uuid.UUID
	OpenTradeRequest *models.OpenTradeRequest
}

func (r ExecuteOpenTradeRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}
