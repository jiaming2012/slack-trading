package services

import (
	"fmt"

	"github.com/jiaming2012/slack-trading/src/go/backtester/models"
)

type LiveAccountSource struct {
	Broker       string             `json:"broker"`
	AccountID    string             `json:"account_id"`
	AccountType  models.AccountRole `json:"account_type"`
	BalancesUrl  string             `json:"-"`
	TradesApiKey string             `json:"-"`
}

func NewLiveAccountSource(broker, accountID, balancesUrl, tradesApiKey string, accountType models.AccountRole) LiveAccountSource {
	return LiveAccountSource{
		Broker:       broker,
		AccountID:    accountID,
		AccountType:  accountType,
		BalancesUrl:  balancesUrl,
		TradesApiKey: tradesApiKey,
	}
}

func (s LiveAccountSource) GetAccountType() models.AccountRole {
	return s.AccountType
}

func (s LiveAccountSource) GetBroker() string {
	return s.Broker
}

func (s LiveAccountSource) GetAccountID() string {
	return s.AccountID
}

func (s LiveAccountSource) GetApiKey() string {
	return s.TradesApiKey
}

func (s LiveAccountSource) GetBrokerUrl() string {
	return s.BalancesUrl
}

func (s LiveAccountSource) Validate() error {
	if s.Broker != "tradier" {
		return fmt.Errorf("unsupported broker: %s", s.Broker)
	}

	return nil
}
