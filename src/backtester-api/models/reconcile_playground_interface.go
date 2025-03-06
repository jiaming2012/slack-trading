package models

import "github.com/google/uuid"

type IReconcilePlayground interface {
	PlaceOrder(order *OrderRecord) ([]*PlaceOrderChanges, []*OrderRecord, error)
	GetOrders() []*OrderRecord
	GetId() uuid.UUID
	GetLiveAccount() ILiveAccount
}
