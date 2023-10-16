package eventservices

import (
	"fmt"
	"github.com/google/uuid"
	"slack-trading/src/eventmodels"
	"slack-trading/src/models"
)

func UpdateExitConditions(accounts []models.Account, newSignalRequest *models.SignalRequest) ([]*models.ExitConditionsSatisfied, error) {
	var aggregatedExitConditionsSatisfied []*models.ExitConditionsSatisfied

	for _, account := range accounts {
		tick := account.Datafeed.Tick()
		for _, strategy := range account.Strategies {
			conditionsAffected := strategy.UpdateExitConditions(newSignalRequest.Name)

			if conditionsAffected > 0 {
				exitConditionsSatisfied, err := strategy.ExitConditionsSatisfied(*tick)
				if err != nil {
					return nil, fmt.Errorf("UpdateExitConditions: strategy.ExitConditionsSatisfied failed %w", err)
				}

				aggregatedExitConditionsSatisfied = append(aggregatedExitConditionsSatisfied, exitConditionsSatisfied...)
			}
		}
	}

	return aggregatedExitConditionsSatisfied, nil
}

// UpdateEntryConditions todo: ideal topology would return (*UpdateConditionsRequest, []*EntryConditionsSatisfied)
// the handler would emit both events if not nil
// this allows updates to not mix with other operations
func UpdateEntryConditions(accounts []models.Account, newSignalRequest *models.SignalRequest) []*models.EntryConditionsSatisfied {
	var entryConditionsSatisfied []*models.EntryConditionsSatisfied

	for _, account := range accounts {
		for _, strategy := range account.Strategies {
			conditionsAffected := strategy.UpdateEntryConditions(newSignalRequest.Name)

			if conditionsAffected > 0 {
				if strategy.EntryConditionsSatisfied() {
					entryConditionsSatisfied = append(entryConditionsSatisfied, models.NewEntryConditionsSatisfied(&account, &strategy))
				}
			}
		}
	}

	return entryConditionsSatisfied
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

		var entryConditions []*models.EntryConditionDTO
		for _, c := range strategy.EntryConditions {
			entryConditions = append(entryConditions, c.ConvertToDTO())
		}

		var exitConditions []*models.ExitConditionDTO
		for _, c := range strategy.ExitConditions {
			exitConditions = append(exitConditions, c.ConvertToDTO())
		}

		statsResult.Strategies = append(statsResult.Strategies, &eventmodels.GetStatsResultItem{
			StrategyName:    strategy.Name,
			Stats:           &stats,
			EntryConditions: entryConditions,
			ExitConditions:  exitConditions,
			OpenTradeLevels: openTradesByPriceLevel,
			CreatedOn:       strategy.CreatedOn,
		})
	}

	return statsResult, nil
}
