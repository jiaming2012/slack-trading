package eventmodels

import (
	"github.com/google/uuid"
)

type ExecuteOpenTradeRequest struct {
	Meta             *MetaData
	RequestID        uuid.UUID
	OpenTradeRequest *OpenTradeRequest
}

func (r ExecuteOpenTradeRequest) GetMetaData() *MetaData {
	return r.Meta
}

func (r ExecuteOpenTradeRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}
