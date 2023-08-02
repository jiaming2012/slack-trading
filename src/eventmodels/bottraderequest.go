package eventmodels

import "slack-trading/src/models"

type BotTradeRequestEvent struct {
	Trade *models.Trade
}
