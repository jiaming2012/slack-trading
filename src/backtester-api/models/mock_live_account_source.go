package models

import "github.com/jiaming2012/slack-trading/src/eventmodels"

type MockLiveAccountSource struct{}

func (m *MockLiveAccountSource) GetBroker() string {
	return "tradier"
}

func (m *MockLiveAccountSource) GetAccountID() string {
	return "mock_default"
}

func (m *MockLiveAccountSource) GetApiKey() string {
	return "mock api key"
}

func (m *MockLiveAccountSource) GetBrokerUrl() string {
	return "mock broker url"
}

func (m *MockLiveAccountSource) GetAccountType() LiveAccountType {
	return LiveAccountTypeMock
}

func (m *MockLiveAccountSource) Validate() error {
	return nil
}

func (m *MockLiveAccountSource) FetchEquity() (*eventmodels.FetchAccountEquityResponse, error) {
	return &eventmodels.FetchAccountEquityResponse{
		Equity:  10000000.00,
		OpenPL:  0.0,
		ClosePL: 0.0,
	}, nil
}

func NewMockLiveAccountSource() *MockLiveAccountSource {
	return &MockLiveAccountSource{}
}
