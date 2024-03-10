package eventmodels

import (
	"github.com/google/uuid"

	"slack-trading/src/models"
)

type ExecuteCloseTradeRequest struct {
	Meta      *MetaData
	RequestID uuid.UUID
	Timeframe *int
	Trade     *models.Trade
	Percent   float64
}

func (r ExecuteCloseTradeRequest) GetMetaData() *MetaData {
	return r.Meta
}

func (r ExecuteCloseTradeRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}

type ExecuteCloseTradesRequest struct {
	Meta               *MetaData
	RequestID          uuid.UUID
	CloseTradesRequest *models.CloseTradesRequest
}

func (r ExecuteCloseTradesRequest) GetMetaData() *MetaData {
	return r.Meta
}

func (r ExecuteCloseTradesRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}
