package models

import "github.com/google/uuid"

type MockDatabase struct{}

func (m *MockDatabase) SaveOrderRecord(playgroundId uuid.UUID, order *BacktesterOrder, liveAccountType LiveAccountType) error {
	return nil
}

func NewMockDatabase() *MockDatabase {
	return &MockDatabase{}
}