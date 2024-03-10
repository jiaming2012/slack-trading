package eventmodels

import (
	"slack-trading/src/models"
	"time"
)

type GetStatsResultItem struct {
	StrategyName    string                      `json:"name"`
	Stats           *models.TradeStats          `json:"stats"`
	EntryConditions []*models.EntryConditionDTO `json:"entryConditions"`
	ExitConditions  []*models.ExitConditionDTO  `json:"exitConditions"`
	OpenTradeLevels []*models.TradeLevels       `json:"openTrades"`
	CreatedOn       time.Time                   `json:"createdOn"`
}
