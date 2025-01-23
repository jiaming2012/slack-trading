package models

type CloseByRequest struct {
	Order    *BacktesterOrder
	Quantity float64
}
