package models

import (
)

type BotTradeRequestEvent struct {
	BaseRequestEvent
	Trade *Trade
}
