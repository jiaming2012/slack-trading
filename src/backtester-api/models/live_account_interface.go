package models

type ILiveAccount interface {
	GetReconcilePlayground() IReconcilePlayground
	PlaceOrder(order *BacktesterOrder) error
}
