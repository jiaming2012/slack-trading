package models

import (
	"context"
	"fmt"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type ILiveAccountSource interface {
	GetBroker() string
	GetAccountID() string
	GetApiKey() string
	GetApiKeyName() string
	GetBrokerUrl() string
	Validate() error
	FetchEquity() (*eventmodels.FetchAccountEquityResponse, error)
}

type LiveAccount struct {
	Balance float64            `json:"balance"`
	Source  ILiveAccountSource `json:"source"`
	Broker  IBroker            `json:"-"`
}

func (a LiveAccount) FetchCurrentPrice(ctx context.Context, symbol eventmodels.Instrument) (float64, error) {
	quotes, err := a.Broker.FetchQuotes(ctx, []eventmodels.Instrument{symbol})
	if err != nil {
		return 0, fmt.Errorf("LiveAccount.FetchCurrentPrice: failed to fetch quotes: %w", err)
	}

	if len(quotes) != 1 {
		return 0, fmt.Errorf("LiveAccount.FetchCurrentPrice: expected 1 quote, got %d", len(quotes))
	}

	return quotes[0].Last, nil
}

func NewLiveAccount(balance float64, source ILiveAccountSource, broker IBroker) *LiveAccount {
	return &LiveAccount{
		Balance: balance,
		Source:  source,
		Broker:  broker,
	}
}
