package models

import "github.com/google/uuid"

type MockLiveAccount struct {
	reconcilePlayground *MockReconcilePlayground
}

func (a *MockLiveAccount) PlaceOrder(account *LiveAccount, order *OrderRecord) ([]*PlaceOrderChanges, error) {
	return nil, nil
}

func (a *MockLiveAccount) GetId() uuid.UUID {
	id, err := uuid.Parse("3b208041-9c52-4221-b514-8d15385d310f")
	if err != nil {
		panic(err)
	}

	return id
}

func NewMockLiveAccount() *MockLiveAccount {
	return &MockLiveAccount{
		reconcilePlayground: &MockReconcilePlayground{},
	}
}
