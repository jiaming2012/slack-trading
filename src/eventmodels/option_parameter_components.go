package eventmodels

type OptionParameterComponents struct {
	Symbol                    StockSymbol `json:"symbol"`
	ExpirationInDays          []int       `json:"expirationInDays"`
	Strikes                   []int       `json:"strikes"`
	MinDistanceBetweenStrikes float64     `json:"minDistanceBetweenStrikes"`
	MaxNoOfStrikes            int         `json:"maxNoOfStrikes"`
	Reason                    string      `json:"reason"`
}
