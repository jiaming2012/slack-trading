package models

import "fmt"

type MockLiveAccount struct {
	reconcilePlayground *MockReconcilePlayground
	broker              IBroker
	database            IDatabaseService
}

func (a *MockLiveAccount) GetDatabase() IDatabaseService {
	return a.database
}

func (a *MockLiveAccount) PlaceOrder(order *OrderRecord) error {
	if err := a.database.SaveOrderRecord(order, nil, false); err != nil {
		return fmt.Errorf("MockLiveAccount.PlaceOrder: failed to save order record: %w", err)
	}

	return nil
}

func (a *MockLiveAccount) SetBroker(broker IBroker) {
	a.broker = broker
}

func (a *MockLiveAccount) GetBroker() IBroker {
	return a.broker
}

func (a *MockLiveAccount) GetId() uint {
	return 12345
}

func NewMockLiveAccount(broker IBroker, database IDatabaseService) *MockLiveAccount {
	return &MockLiveAccount{
		reconcilePlayground: &MockReconcilePlayground{},
		broker:              broker,
		database:            database,
	}
}
