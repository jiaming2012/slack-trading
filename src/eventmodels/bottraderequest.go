package eventmodels

import (
	"github.com/google/uuid"

	"slack-trading/src/models"
)

type BotTradeRequestEvent struct {
	RequestID uuid.UUID
	Trade     *models.Trade
}

func (ev BotTradeRequestEvent) GetRequestID() uuid.UUID {
	return ev.RequestID
}
