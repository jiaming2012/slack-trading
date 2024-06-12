package eventmodels

type StrategiesPostRequest struct {
	Name            string              `json:"name"`
	Balance         float64             `json:"balance"`
	Symbol          string              `json:"symbol"`
	Direction       Direction           `json:"direction"`
	EntryConditions []EntryConditionDTO `json:"entryConditions"`
	ExitConditions  []ExitConditionDTO  `json:"exitConditions"`
	PriceLevels     []PriceLevelDTO     `json:"priceLevels"`
}
