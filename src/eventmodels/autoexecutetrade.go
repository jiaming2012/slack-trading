package eventmodels

import (
	"github.com/google/uuid"
	"slack-trading/src/models"
)

type AutoExecuteTrade struct {
	RequestID uuid.UUID
	Trade     *models.Trade
}
