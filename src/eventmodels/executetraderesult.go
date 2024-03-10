package eventmodels

import (
	"github.com/google/uuid"

	"slack-trading/src/models"
)

type ExecuteOpenTradeResult struct {
	Meta      *MetaData                      `json:"meta"`
	RequestID uuid.UUID                      `json:"id"`
	Side      string                         `json:"side"`
	Result    *models.ExecuteOpenTradeResult `json:"result"`
}

func (r *ExecuteOpenTradeResult) GetMetaData() *MetaData {
	return r.Meta
}

func (r *ExecuteOpenTradeResult) GetRequestID() uuid.UUID {
	return r.RequestID
}

type ExecuteCloseTradesResult struct {
	Meta      *MetaData                      `json:"meta"`
	RequestID uuid.UUID                      `json:"id"`
	Side      string                         `json:"side"`
	Result    *models.ExecuteOpenTradeResult `json:"result"`
}

func (r *ExecuteCloseTradesResult) GetMetaData() *MetaData {
	return r.Meta
}

func (r *ExecuteCloseTradesResult) GetRequestID() uuid.UUID {
	return r.RequestID
}
