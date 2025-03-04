package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type LiveAccount struct {
	gorm.Model
	Source                     ILiveAccountSource   `json:"source" gorm:"-"`
	Broker                     IBroker              `json:"-" gorm:"-"`
	ReconcilePlayground        IReconcilePlayground `json:"-" gorm:"-"`
	ReconcilePlaygroundID      uuid.UUID            `json:"reconcile_playground_id" gorm:"column:reconcile_playground_id;type:uuid;index:idx_live_account_reconcile_playground_id"`
	ReconcilePlaygroundSession PlaygroundSession    `json:"-" gorm:"foreignKey:ReconcilePlaygroundID;references:ID"`
	BrokerName                 string               `gorm:"column:broker;type:text"`
	AccountId                  string               `gorm:"column:account_id;type:text"`
	AccountType                LiveAccountType      `gorm:"column:account_type;type:text"`
	PlotUpdatedAt              time.Time            `gorm:"column:plot_updated_at;type:timestamptz"`
}

func (a *LiveAccount) GetReconcilePlayground() IReconcilePlayground {
	return a.ReconcilePlayground
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

func (a *LiveAccount) PlaceOrder(order *BacktesterOrder) error {
	ticker := order.Symbol.GetTicker()
	qty := int(order.AbsoluteQuantity)
	req := NewPlaceEquityOrderRequest(ticker, qty, order.Side, order.Type, order.Tag, false)

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
				order.ID = uint(id)
			} else {
				return fmt.Errorf("LivePlayground.PlaceOrder: failed to cast order id to int")
			}
		} else {
			return fmt.Errorf("LivePlayground.PlaceOrder: order id not found in response")
		}
	}

	return nil
}

func NewLiveAccount(source ILiveAccountSource, broker IBroker, reconcilePlayground IReconcilePlayground) (*LiveAccount, error) {
	if err := reconcilePlayground.SetBroker(broker); err != nil {
		return nil, fmt.Errorf("NewLiveAccount: failed to set broker: %w", err)
	}

	return &LiveAccount{
		Source:              source,
		Broker:              broker,
		ReconcilePlayground: reconcilePlayground,
		BrokerName:          source.GetBroker(),
		AccountId:           source.GetAccountID(),
		AccountType:         source.GetAccountType(),
	}, nil
}
