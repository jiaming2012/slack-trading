package eventmodels

type OptionParameterComponents struct {
	Symbol                    string  `json:"symbol"`
	ExpirationInDays          []int   `json:"expirationInDays"`
	Strikes                   []int   `json:"strikes"`
	MinDistanceBetweenStrikes float64 `json:"minDistanceBetweenStrikes"`
	MaxNoOfStrikes            int     `json:"maxNoOfStrikes"`
	Reason                    string  `json:"reason"`
}
