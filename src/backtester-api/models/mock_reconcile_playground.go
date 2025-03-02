package models

import "github.com/google/uuid"

type MockReconcilePlayground struct {
	broker IBroker
	orders []*BacktesterOrder
}

func (p *MockReconcilePlayground) GetOrders() []*BacktesterOrder {
	return p.orders
}

func (p *MockReconcilePlayground) PlaceOrder(account ILiveAccount, order *BacktesterOrder) ([]*PlaceOrderChanges, []*BacktesterOrder, error) {
	p.orders = append(p.orders, order)
	return nil, nil, nil
}

func (p *MockReconcilePlayground) SetBroker(broker IBroker) error {
	p.broker = broker
	return nil
}

func (p *MockReconcilePlayground) GetId() uuid.UUID {
	id, err := uuid.Parse("c966a59f-1183-4fe6-85f9-b09b7dcd23f4")
	if err != nil {
		panic(err)
	}

	return id
}
