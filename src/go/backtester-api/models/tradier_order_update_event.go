package models

type TradierOrderUpdateEvent struct {
	CreateOrder *TradierOrderCreateEvent
	ModifyOrder *TradierOrderModifyEvent
	DeleteOrder *TradierOrderDeleteEvent
}
