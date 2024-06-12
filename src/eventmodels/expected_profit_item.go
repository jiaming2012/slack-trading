package eventmodels

type ExpectedProfitItem struct {
	Description    string   `json:"description"`
	DebitPaid      *float64 `json:"debit_paid"`
	CreditReceived *float64 `json:"credit_received"`
	ExpectedProfit float64  `json:"expected_profit"`
}