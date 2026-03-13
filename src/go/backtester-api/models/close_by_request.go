package models

type CloseByRequest struct {
	Order    *OrderRecord
	Quantity float64
}
