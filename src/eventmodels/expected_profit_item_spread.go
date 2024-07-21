package eventmodels

type ExpectedProfitItemSpread struct {
	Description             string     `json:"description"`
	Type                    OptionType `json:"type"`
	LongOptionSymbol        string     `json:"long_option_symbol"`
	LongOptionExpiration    string     `json:"long_option_expiration"`
	LongOptionAvgFillPrice  float64    `json:"long_option_avg_fill_price"`
	ShortOptionSymbol       string     `json:"short_option_symbol"`
	ShortOptionExpiration   string     `json:"short_option_expiration"`
	ShortOptionAvgFillPrice float64    `json:"short_option_avg_fill_price"`
	DebitPaid               *float64   `json:"debit_paid"`
	CreditReceived          *float64   `json:"credit_received"`
	ExpectedProfit          float64    `json:"expected_profit"`
}
