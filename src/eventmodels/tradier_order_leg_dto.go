package eventmodels

import "time"

type TradierOrderLegDTO struct {
	ID                uint      `json:"id"`
	Type              string    `json:"type"`
	Symbol            string    `json:"symbol"`
	Side              string    `json:"side"`
	Quantity          float64   `json:"quantity"`
	Status            string    `json:"status"`
	Duration          string    `json:"duration"`
	AvgFillPrice      float64   `json:"avg_fill_price"`
	ExecQuantity      float64   `json:"exec_quantity"`
	LastFillPrice     float64   `json:"last_fill_price"`
	LastFillQuantity  float64   `json:"last_fill_quantity"`
	RemainingQuantity float64   `json:"remaining_quantity"`
	CreateDate        time.Time `json:"create_date"`
	TransactionDate   time.Time `json:"transaction_date"`
	Class             string    `json:"class"`
	OptionSymbol      string    `json:"option_symbol"`
}
