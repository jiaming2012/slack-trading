package eventmodels

import (
	"github.com/google/uuid"
)

type ExecuteCloseTradeRequest struct {
	Meta      *MetaData
	RequestID uuid.UUID
	Timeframe *int
	Trade     *Trade
	Percent   float64
}

func (r ExecuteCloseTradeRequest) GetMetaData() *MetaData {
	return r.Meta
}

func (r ExecuteCloseTradeRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}

type ExecuteCloseTradesRequest struct {
	BaseRequestEvent2
	CloseTradesRequest *CloseTradesRequest
}
