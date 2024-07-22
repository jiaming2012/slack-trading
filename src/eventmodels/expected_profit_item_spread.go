package eventmodels

import "time"

type ExpectedProfitItemSpread struct {
	Description             string     `json:"description"`
	Type                    OptionType `json:"type"`
	LongOptionTimestamp     time.Time  `json:"long_option_timestamp"`
	LongOptionSymbol        string     `json:"long_option_symbol"`
	LongOptionExpiration    string     `json:"long_option_expiration"`
	LongOptionAvgFillPrice  float64    `json:"long_option_avg_fill_price"`
	ShortOptionTimestamp    time.Time  `json:"short_option_timestamp"`
	ShortOptionSymbol       string     `json:"short_option_symbol"`
	ShortOptionExpiration   string     `json:"short_option_expiration"`
	ShortOptionAvgFillPrice float64    `json:"short_option_avg_fill_price"`
	DebitPaid               *float64   `json:"debit_paid"`
	CreditReceived          *float64   `json:"credit_received"`
	ExpectedProfit          float64    `json:"expected_profit"`
}
