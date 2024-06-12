package eventmodels

import (
	"time"
)

type GetStatsResultItem struct {
	StrategyName    string               `json:"name"`
	Stats           *TradeStats          `json:"stats"`
	EntryConditions []*EntryConditionDTO `json:"entryConditions"`
	ExitConditions  []*ExitConditionDTO  `json:"exitConditions"`
	OpenTradeLevels []*TradeLevels       `json:"openTrades"`
	CreatedOn       time.Time            `json:"createdOn"`
}
