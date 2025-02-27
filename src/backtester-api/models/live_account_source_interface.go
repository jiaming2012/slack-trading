package models

import "github.com/jiaming2012/slack-trading/src/eventmodels"

type ILiveAccountSource interface {
	GetBroker() string
	GetAccountID() string
	GetApiKey() string
	GetBrokerUrl() string
	GetAccountType() LiveAccountType
	Validate() error
	FetchEquity() (*eventmodels.FetchAccountEquityResponse, error)
}
