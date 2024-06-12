package eventmodels

import (
	"slack-trading/src/models"
)

type BotTradeRequestEvent struct {
	BaseRequestEvent
	Trade *models.Trade
}
