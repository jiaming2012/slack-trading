package models

type ILiveAccountSource interface {
	GetBroker() string
	GetAccountID() string
	GetApiKey() string
	GetBrokerUrl() string
	GetAccountType() AccountRole
	Validate() error
}
