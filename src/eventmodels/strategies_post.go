package eventmodels

import "slack-trading/src/models"

type StrategiesPostRequest struct {
	Name            string                     `json:"name"`
	Balance         float64                    `json:"balance"`
	Symbol          string                     `json:"symbol"`
	Direction       models.Direction           `json:"direction"`
	EntryConditions []models.EntryConditionDTO `json:"entryConditions"`
	ExitConditions  []models.ExitConditionDTO  `json:"exitConditions"`
	PriceLevels     []models.PriceLevelDTO     `json:"priceLevels"`
}
