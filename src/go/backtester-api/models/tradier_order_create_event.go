package models

import "github.com/jiaming2012/slack-trading/src/go/models"

type TradierOrderCreateEvent struct {
	Order               *models.TradierOrder
	OrderRecord         *OrderRecord
	ReconcilePlayground IReconcilePlayground
}
