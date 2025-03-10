package models

import (
	"time"
)

type ExecutionFillRequest struct {
	ReconcilePlayground IReconcilePlayground
	OrderRecord         *OrderRecord
	Price               float64
	Quantity            float64
	Time                time.Time
}
