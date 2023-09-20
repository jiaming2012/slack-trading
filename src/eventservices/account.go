package eventservices

import (
	"fmt"
	"github.com/google/uuid"
	"slack-trading/src/eventmodels"
	"slack-trading/src/models"
)

// UpdateConditions todo: ideal topology would return (*UpdateConditionsRequest, []*EntryConditionsSatisfied)
// the handler would emit both events if not nil
// this allows updates to not mix with other operations
func UpdateConditions(accounts []models.Account, newSignalRequest *eventmodels.SignalRequest) (*eventmodels.NewSignalResult, []*eventmodels.EntryConditionsSatisfied) {
	var entryConditionsSatisfied []*eventmodels.EntryConditionsSatisfied

	for _, account := range accounts {
		for _, strategy := range account.Strategies {
			conditionsAffected := strategy.UpdateEntryConditions(newSignalRequest.Name)

			if conditionsAffected > 0 {
				if strategy.EntryConditionsSatisfied() {
					entryConditionsSatisfied = append(entryConditionsSatisfied, eventmodels.NewEntryConditionsSatisfied(&account, &strategy))
				}
			}
		}
	}

	return &eventmodels.NewSignalResult{
		RequestID: newSignalRequest.RequestID,
		Name:      newSignalRequest.Name,
	}, entryConditionsSatisfied
}

func FetchTrades(requestID uuid.UUID, account *models.Account) *eventmodels.FetchTradesResult {
	priceLevelTrades := account.GetPriceLevelTrades(false)
	return eventmodels.NewFetchTradesResult(requestID, priceLevelTrades)
}

func GetStats(requestID uuid.UUID, account *models.Account, currentTick *models.Tick) (*eventmodels.GetStatsResult, error) {
	statsResult := &eventmodels.GetStatsResult{
		RequestID: requestID,
	}

	for _, strategy := range account.Strategies {
		stats, statsErr := strategy.GetTrades().GetTradeStats(*currentTick)
		if statsErr != nil {
			return nil, fmt.Errorf("GetStats: failed to get trade stats: %w", statsErr)
		}

		openTradesByPriceLevel := strategy.GetTradesByPriceLevel(true)

		statsResult.Strategies = append(statsResult.Strategies, &eventmodels.GetStatsResultItem{
			StrategyName:    strategy.Name,
			Stats:           &stats,
			EntryConditions: strategy.EntryConditions,
			OpenTradeLevels: openTradesByPriceLevel,
		})
	}

	return statsResult, nil
}
