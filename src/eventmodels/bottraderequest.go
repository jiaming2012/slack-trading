package eventmodels

import (
	"github.com/jiaming2012/slack-trading/src/models"
)

type BotTradeRequestEvent struct {
	BaseRequestEvent
	Trade *models.Trade
}
