package models

import "github.com/jiaming2012/slack-trading/src/eventmodels"

type TradierOrderCreateEvent struct {
	Order               *eventmodels.TradierOrder
	OrderRecord         *OrderRecord
	ReconcilePlayground IReconcilePlayground
}
