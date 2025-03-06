package models

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type LiveAccount struct {
	gorm.Model
	// ReconcilePlaygroundID      uuid.UUID                                 `json:"reconcile_playground_id" gorm:"column:reconcile_playground_id;type:uuid;index:idx_live_account_reconcile_playground_id"`
	// ReconcilePlaygroundSession *Playground                               `json:"-" gorm:"foreignKey:ReconcilePlaygroundID;references:ID"`
	BrokerName    string          `gorm:"column:broker;type:text"`
	AccountId     string          `gorm:"column:account_id;type:text"`
	AccountType   LiveAccountType `gorm:"column:account_type;type:text"`
	PlotUpdatedAt time.Time       `gorm:"column:plot_updated_at;type:timestamptz"`
	Broker        IBroker         `json:"-" gorm:"-"`
	// ReconcilePlayground        IReconcilePlayground                      `json:"-" gorm:"-"`
	database IDatabaseService `json:"-" gorm:"-"`
}

func (a *LiveAccount) GetSource() CreateAccountRequestSource {
	return CreateAccountRequestSource{
		Broker:     a.BrokerName,
		AccountID:  a.AccountId,
		AccountType: a.AccountType,
	}
}

func (a *LiveAccount) GetId() uint {
	return a.ID
}

func (a *LiveAccount) GetDatabase() IDatabaseService {
	return a.database
}

func (a *LiveAccount) SetDatabase(database IDatabaseService) {
	a.database = database
}

func (a *LiveAccount) SetBroker(broker IBroker) {
	a.Broker = broker
}

func (a *LiveAccount) GetBroker() IBroker {
	return a.Broker
}

func (a *LiveAccount) FetchCurrentPrice(ctx context.Context, symbol eventmodels.Instrument) (float64, error) {
	quotes, err := a.Broker.FetchQuotes(ctx, []eventmodels.Instrument{symbol})
	if err != nil {
		return 0, fmt.Errorf("LiveAccount.FetchCurrentPrice: failed to fetch quotes: %w", err)
	}

	if len(quotes) != 1 {
		return 0, fmt.Errorf("LiveAccount.FetchCurrentPrice: expected 1 quote, got %d", len(quotes))
	}

	return quotes[0].Last, nil
}

func (a *LiveAccount) PlaceOrder(order *OrderRecord) error {
	ticker := order.GetInstrument().GetTicker()
	qty := int(order.AbsoluteQuantity)
	req := NewPlaceEquityOrderRequest(ticker, qty, order.Side, order.OrderType, order.Tag, false)

	resp, err := a.Broker.PlaceOrder(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to place order in live playground: %w", err)
	}

	if orderMap, ok := resp["order"]; ok {
		result, ok := orderMap.(map[string]interface{})
		if !ok {
			return fmt.Errorf("LivePlayground.PlaceOrder: failed to cast response to order id map")
		}

		if orderID, ok := result["id"]; ok {
			if id, ok := orderID.(float64); ok {
				val := uint(id)
				order.ExternalOrderID = &val
			} else {
				return fmt.Errorf("LivePlayground.PlaceOrder: failed to cast order id to int")
			}
		} else {
			return fmt.Errorf("LivePlayground.PlaceOrder: order id not found in response")
		}
	}

	return nil
}

func NewLiveAccount(broker IBroker, database IDatabaseService) (*LiveAccount, error) {
	return &LiveAccount{
		Broker:      broker,
		BrokerName:  broker.GetSource().GetBroker(),
		AccountId:   broker.GetSource().GetAccountID(),
		AccountType: broker.GetSource().GetAccountType(),
		database:    database,
	}, nil
}
