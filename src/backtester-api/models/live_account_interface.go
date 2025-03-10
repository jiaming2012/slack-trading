package models

type ILiveAccount interface {
	GetId() uint
	PlaceOrder(order *OrderRecord) error
	GetBroker() IBroker
	SetBroker(broker IBroker)
	GetDatabase() IDatabaseService
}
