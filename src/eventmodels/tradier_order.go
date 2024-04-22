package eventmodels

import (
	"fmt"
	"time"
)

type TradierOrder struct {
	ID                uint64    `json:"id"`
	Type              string    `json:"type"`
	Symbol            string    `json:"symbol"`
	Side              string    `json:"side"`
	Quantity          float64   `json:"quantity"`
	Status            string    `json:"status"`
	Duration          string    `json:"duration"`
	Price             float64   `json:"price"`
	AvgFillPrice      float64   `json:"avg_fill_price"`
	ExecQuantity      float64   `json:"exec_quantity"`
	LastFillPrice     float64   `json:"last_fill_price"`
	LastFillQuantity  float64   `json:"last_fill_quantity"`
	RemainingQuantity float64   `json:"remaining_quantity"`
	CreateDate        time.Time `json:"create_date"`
	TransactionDate   time.Time `json:"transaction_date"`
	Class             string    `json:"class"`
	OptionSymbol      *string   `json:"option_symbol"`
}

func (o TradierOrder) String() string {
	var symbol string
	if o.OptionSymbol != nil {
		symbol = *o.OptionSymbol
	} else {
		symbol = o.Symbol
	}

	timestamp := o.CreateDate.Format("2006-01-02 15:04:05")
	return fmt.Sprintf("ID (%d), Type: %s, Symbol: %s, Side: %s, Status: %s, AvgFillPrice: %.2f, ExecQuantity: %.0f, Class: %s, CreatedAt: %v", o.ID, o.Type, symbol, o.Side, o.Status, o.AvgFillPrice, o.ExecQuantity, o.Class, timestamp)
}
