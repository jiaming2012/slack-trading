package eventmodels

import (
	"github.com/jiaming2012/slack-trading/src/go/models"
)

type BotTradeRequestEvent struct {
	BaseRequestEvent
	Trade *models.Trade
}
