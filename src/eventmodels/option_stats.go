package eventmodels

type OptionStats struct {
	ExpectedProfitLong  *float64 `json:"expected_profit_long"`
	ExpectedProfitShort *float64 `json:"expected_profit_short"`
}
