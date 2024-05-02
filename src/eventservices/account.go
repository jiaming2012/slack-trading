package eventservices

import (
	"fmt"

	"github.com/google/uuid"

	"slack-trading/src/eventmodels"
)

func UpdateExitConditions(accounts []*eventmodels.Account, newSignalRequest *eventmodels.CreateSignalRequestEventV1) ([]*eventmodels.ExitConditionsSatisfied, error) {
	var aggregatedExitConditionsSatisfied []*eventmodels.ExitConditionsSatisfied

	for _, account := range accounts {
		tick := account.Datafeed.Tick()
		for _, strategy := range account.Strategies {
			conditionsAffected := strategy.UpdateExitConditions(newSignalRequest)

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
func UpdateEntryConditions(accounts []*eventmodels.Account, newSignalRequest *eventmodels.CreateSignalRequestEventV1) []*eventmodels.EntryConditionsSatisfied {
	var entryConditionsSatisfied []*eventmodels.EntryConditionsSatisfied

	for _, account := range accounts {
		for _, strategy := range account.Strategies {
			conditionsAffected := strategy.UpdateEntryConditions(newSignalRequest)

			if conditionsAffected > 0 {
				if strategy.EntryConditionsSatisfied() {
					entryConditionsSatisfied = append(entryConditionsSatisfied, eventmodels.NewEntryConditionsSatisfied(account, strategy))
				}
			}
		}
	}

	return entryConditionsSatisfied
}

func FetchTrades(requestID uuid.UUID, account *eventmodels.Account) *eventmodels.FetchTradesResult {
	priceLevelTrades := account.GetPriceLevelTrades(false)
	return eventmodels.NewFetchTradesResult(requestID, priceLevelTrades)
}

func GetStats(requestID uuid.UUID, account *eventmodels.Account, currentTick *eventmodels.Tick) (*eventmodels.GetStatsResult, error) {
	statsResult := &eventmodels.GetStatsResult{}

	for _, strategy := range account.Strategies {
		stats, statsErr := strategy.GetTrades().GetTradeStats(*currentTick)
		if statsErr != nil {
			return nil, fmt.Errorf("GetStats: failed to get trade stats: %w", statsErr)
		}

		openTradesByPriceLevel := strategy.GetTradesByPriceLevel(true)

		var entryConditions []*eventmodels.EntryConditionDTO
		for _, c := range strategy.EntryConditions {
			entryConditions = append(entryConditions, c.ConvertToDTO())
		}

		var exitConditions []*eventmodels.ExitConditionDTO
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
