package eventmodels

import (
	"github.com/google/uuid"
)

type ExecuteOpenTradeRequest struct {
	ParentRequest    interface{}
	Meta             *MetaData
	RequestID        uuid.UUID
	OpenTradeRequest *CreateTradeRequest
}

func (r ExecuteOpenTradeRequest) GetMetaData() *MetaData {
	return r.Meta
}

func (r ExecuteOpenTradeRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}
