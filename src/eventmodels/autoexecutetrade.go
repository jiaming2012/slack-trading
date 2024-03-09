package eventmodels

import (
	"slack-trading/src/models"

	"github.com/google/uuid"
)

type AutoExecuteTrade struct {
	RequestID uuid.UUID
	Trade     *models.Trade
}

func (r AutoExecuteTrade) GetRequestID() uuid.UUID {
	return r.RequestID
}
