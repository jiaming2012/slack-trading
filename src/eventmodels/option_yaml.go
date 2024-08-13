package eventmodels

type OptionYAML struct {
	Symbol                             string   `yaml:"symbol"`
	StartsAt                           string   `yaml:"startsAt"`
	EndsAt                             string   `yaml:"endsAt"`
	ExpirationsInDays                  []int    `yaml:"expirationsInDays"`
	MinDistanceBetweenStrikes          *float64 `yaml:"minDistanceBetweenStrikes,omitempty"`
	MinStandardDeviationBetweenStrikes *float64 `yaml:"minStandardDeviationBetweenStrikes,omitempty"`
	MaxNoOfStrikes                     int      `yaml:"maxNoOfStrikes"`
	MaxNoOfPositions                   int      `yaml:"maxNoOfPositions"`
}
