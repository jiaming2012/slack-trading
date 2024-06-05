package eventmodels

type ExpectedProfitItemSpreadDTO struct {
	Description       string     `json:"description"`
	Type              OptionType `json:"type"`
	LongOptionSymbol  string     `json:"long_option_symbol"`
	ShortOptionSymbol string     `json:"short_option_symbol"`
	DebitPaid         *float64   `json:"debit_paid"`
	CreditReceived    *float64   `json:"credit_received"`
	ExpectedProfit    float64    `json:"expected_profit"`
}
