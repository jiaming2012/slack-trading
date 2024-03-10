package eventmodels

import (
	"github.com/google/uuid"

	"slack-trading/src/models"
)

type ExecuteOpenTradeRequest struct {
	Meta             *MetaData
	RequestID        uuid.UUID
	OpenTradeRequest *models.OpenTradeRequest
}

func (r ExecuteOpenTradeRequest) GetMetaData() *MetaData {
	return r.Meta
}

func (r ExecuteOpenTradeRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}
