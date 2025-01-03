package models

import "github.com/jiaming2012/slack-trading/src/eventmodels"

type ILiveAccountSource interface {
	GetBroker() string
	GetAccountID() string
	GetApiKey() string
	Validate() error
	FetchEquity() (*eventmodels.FetchAccountEquityResponse, error)
}

type LiveAccount struct {
	Balance float64            `json:"balance"`
	Source  ILiveAccountSource `json:"source"`
}

func NewLiveAccount(balance float64, source ILiveAccountSource) *LiveAccount {
	return &LiveAccount{
		Balance: balance,
		Source:  source,
	}
}
