package models

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type ILiveAccountSource interface {
	GetBroker() string
	GetAccountID() string
	GetApiKey() string
	GetBrokerUrl() string
	GetAccountType() *LiveAccountType
	Validate() error
	FetchEquity() (*eventmodels.FetchAccountEquityResponse, error)
}

type LiveAccount struct {
	gorm.Model
	Source        ILiveAccountSource `json:"source" gorm:"-"`
	Broker        IBroker            `json:"-" gorm:"-"`
	BrokerName    string             `gorm:"column:broker;type:text"`
	AccountId     string             `gorm:"column:account_id;type:text"`
	AccountType   string             `gorm:"column:account_type;type:text"`
	PlotUpdatedAt time.Time          `gorm:"column:plot_updated_at;type:timestamptz"`
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

func NewLiveAccount(source ILiveAccountSource, broker IBroker) *LiveAccount {
	return &LiveAccount{
		Source: source,
		Broker: broker,
	}
}
