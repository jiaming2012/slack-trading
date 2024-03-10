package eventmodels

import (
	"github.com/google/uuid"

	"slack-trading/src/models"
)

type AutoExecuteTrade struct {
	Meta      *MetaData
	RequestID uuid.UUID
	Trade     *models.Trade
}

func (r AutoExecuteTrade) GetMetaData() *MetaData {
	return r.Meta
}

func (r AutoExecuteTrade) GetRequestID() uuid.UUID {
	return r.RequestID
}
