package models

import "github.com/google/uuid"

type IReconcilePlayground interface {
	PlaceOrder(account ILiveAccount, order *BacktesterOrder) ([]*PlaceOrderChanges, error)
	SetBroker(broker IBroker) error
	GetId() uuid.UUID
}
