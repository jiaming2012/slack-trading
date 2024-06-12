package eventmodels

type ExpectedProfitItemSpread struct {
	Description           string     `json:"description"`
	Type                  OptionType `json:"type"`
	LongOptionSymbol      string     `json:"long_option_symbol"`
	LongOptionExpiration  string     `json:"long_option_expiration"`
	ShortOptionSymbol     string     `json:"short_option_symbol"`
	ShortOptionExpiration string     `json:"short_option_expiration"`
	DebitPaid             *float64   `json:"debit_paid"`
	CreditReceived        *float64   `json:"credit_received"`
	ExpectedProfit        float64    `json:"expected_profit"`
}