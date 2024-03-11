package eventmodels

import (
	"github.com/google/uuid"
)

type AutoExecuteTrade struct {
	Meta      *MetaData
	RequestID uuid.UUID
	Trade     *Trade
}

func (r AutoExecuteTrade) GetMetaData() *MetaData {
	return r.Meta
}

func (r AutoExecuteTrade) GetRequestID() uuid.UUID {
	return r.RequestID
}
