package eventmodels

import "time"

type ExpectedProfitItemCondor struct {
	Description         string     `json:"description"`
	Type                OptionType `json:"type"`
	LongCallTimestamp time.Time  `json:"long_call_timestamp"`
	LongCallSymbol      string     `json:"long_call_symbol"`
	LongCallExpiration  string     `json:"long_call_expiration"`
	LongCallAvgFillPrice float64    `json:"long_call_avg_fill_price"`
	LongPutTimestamp    time.Time  `json:"long_put_timestamp"`
	LongPutSymbol       string     `json:"long_put_symbol"`
	LongPutExpiration   string     `json:"long_put_expiration"`
	LongPutAvgFillPrice  float64    `json:"long_put_avg_fill_price"`
	ShortCallTimestamp time.Time  `json:"short_call_timestamp"`
	ShortCallSymbol     string     `json:"short_call_symbol"`
	ShortCallExpiration string     `json:"short_call_expiration"`
	ShortCallAvgFillPrice float64    `json:"short_call_avg_fill_price"`
	ShortPutTimestamp    time.Time  `json:"short_put_timestamp"`
	ShortPutSymbol      string     `json:"short_put_symbol"`
	ShortPutExpiration  string     `json:"short_put_expiration"`
	ShortPutAvgFillPrice float64    `json:"short_put_avg_fill_price"`
	CreditReceived      *float64   `json:"credit_received"`
	DebitPaid           *float64   `json:"debit_paid"`
	ExpectedProfit      float64    `json:"expected_profit"`
}
