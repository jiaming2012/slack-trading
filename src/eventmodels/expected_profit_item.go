package eventmodels

type ExpectedProfitItem struct {
	Description    string  `json:"description"`
	Premium        float64 `json:"premium"`
	ExpectedProfit float64 `json:"expected_profit"`
}
