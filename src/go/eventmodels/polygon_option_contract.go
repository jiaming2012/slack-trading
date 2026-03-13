package eventmodels

type PolygonOptionContract struct {
	ContractType      OptionType   `json:"contract_type"`
	ExerciseStyle     string       `json:"exercise_style"`
	ExpirationDate    string       `json:"expiration_date"`
	SharesPerContract int          `json:"shares_per_contract"`
	StrikePrice       float64      `json:"strike_price"`
	Ticker            OptionSymbol `json:"ticker"`
	UnderlyingTicker  StockSymbol  `json:"underlying_ticker"`
}
