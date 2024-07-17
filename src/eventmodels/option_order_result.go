package eventmodels

import "time"

type OptionOrderSpreadResult struct {
	OrderID            uint         `json:"order_id" csv:"order_id"`
	Underlying         string       `json:"underlying" csv:"underlying"`
	ExecutionType      string       `json:"execution_type" csv:"execution_type"`
	Strategy           string       `json:"strategy" csv:"strategy"`
	CreatedTimestamp   time.Time    `json:"created_timestamp" csv:"created_timestamp"`
	RequestedPrice     float64      `json:"requested_price" csv:"requested_price"`
	ExecutedPrice      float64      `json:"executed_price" csv:"executed_price"`
	Slippage           float64      `json:"slippage" csv:"slippage"`
	DebitPaid          float64      `json:"debit_paid" csv:"debit_paid"`
	CreditReceived     float64      `json:"credit_received" csv:"credit_received"`
	OrderID1           uint         `json:"order_id_1" csv:"order_id_1"`
	Side1              string       `json:"side_1" csv:"side_1"`
	OptionType1        OptionType   `json:"option_type_1" csv:"option_type_1"`
	Symbol1            OptionSymbol `json:"symbol_1" csv:"symbol_1"`
	Quantity1          float64      `json:"quantity_1" csv:"quantity_1"`
	AvgFillPrice1      float64      `json:"avg_fill_price_1" csv:"avg_fill_price_1"`
	StrikePrice1       float64      `json:"strike_price_1" csv:"strike_price_1"`
	InTheMoney1        bool         `json:"in_the_money_1" csv:"in_the_money_1"`
	Profit1            float64      `json:"profit_1" csv:"profit_1"`
	OrderID2           uint         `json:"order_id_2" csv:"order_id_2"`
	Side2              string       `json:"side_2" csv:"side_2"`
	OptionType2        OptionType   `json:"option_type_2" csv:"option_type_2"`
	Symbol2            OptionSymbol `json:"symbol_2" csv:"symbol_2"`
	Quantity2          float64      `json:"quantity_2" csv:"quantity_2"`
	AvgFillPrice2      float64      `json:"avg_fill_price_2" csv:"avg_fill_price_2"`
	StrikePrice2       float64      `json:"strike_price_2" csv:"strike_price_2"`
	InTheMoney2        bool         `json:"in_the_money_2" csv:"in_the_money_2"`
	Profit2            float64      `json:"profit_2" csv:"profit_2"`
	SignalName         string       `json:"signal_name" csv:"signal_name"`
	ExpirationDate     time.Time    `json:"expiration_date" csv:"expiration_date"`
	PriceAtExpiry      float64      `json:"price_at_expiry" csv:"price_at_expiry"`
	ExpectedProfit     float64      `json:"expected_profit" csv:"expected_profit"`
	Profit             float64      `json:"profit" csv:"profit"`
	MaxProfit          float64      `json:"max_profit" csv:"max_profit"`
	MaxProfitTimestamp time.Time    `json:"max_profit_timestamp" csv:"max_profit_timestamp"`
	IsClosed           bool         `json:"is_closed" csv:"is_closed"`
}
