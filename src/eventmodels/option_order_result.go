package eventmodels

import "time"

type OptionOrderSpreadResult struct {
	Underlying         string    `json:"underlying"`
	ExecutionType      string    `json:"execution_type"`
	Strategy           string    `json:"strategy"`
	CreatedTimestamp   time.Time `json:"created_timestamp"`
	OrderID1           uint      `json:"order_id_1"`
	Symbol1            string    `json:"symbol_1"`
	Type1              string    `json:"type_1"`
	Quantity1          float64   `json:"quantity_1"`
	AvgFillPrice1      float64   `json:"avg_fill_price_1"`
	OrderID2           uint      `json:"order_id_2"`
	Symbol2            string    `json:"symbol_2"`
	Type2              string    `json:"type_2"`
	Quantity2          float64   `json:"quantity_2"`
	AvgFillPrice2      float64   `json:"avg_fill_price_2"`
	SignalName         string    `json:"signal_name"`
	ExpirationDate     time.Time `json:"expiration_date"`
	ExpectedProfit     float64   `json:"expected_profit"`
	RequestedPrice     float64   `json:"requested_price"`
	ExecutedPrice      float64   `json:"executed_price"`
	PriceAtExpiry      float64   `json:"price_at_expiry"`
	Profit             float64   `json:"profit"`
	MaxProfit          float64   `json:"max_profit"`
	MaxProfitTimestamp time.Time `json:"max_profit_timestamp"`
	IsClosed           bool      `json:"is_closed"`
}
