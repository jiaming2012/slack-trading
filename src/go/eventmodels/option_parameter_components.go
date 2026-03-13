package eventmodels

type OptionParameterComponents struct {
	StockSymbol               StockSymbol `json:"stockSymbol"`
	FxSymbol                  FxSymbol    `json:"fxSymbol"`
	ExpirationInDays          []int       `json:"expirationInDays"`
	Strikes                   []int       `json:"strikes"`
	MinDistanceBetweenStrikes float64     `json:"minDistanceBetweenStrikes"`
	MaxNoOfStrikes            int         `json:"maxNoOfStrikes"`
	Reason                    string      `json:"reason"`
}
