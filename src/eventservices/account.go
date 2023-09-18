package eventservices

import (
	"fmt"
	"github.com/google/uuid"
	"slack-trading/src/eventmodels"
	"slack-trading/src/models"
)

func UpdateConditions(accounts []models.Account, newSignalRequest *eventmodels.NewSignalRequest) *eventmodels.NewSignalResult {
	strategiesAffected := 0

	for _, account := range accounts {
		for _, strategy := range account.Strategies {
			strategyAffected := false
			for _, condition := range strategy.Conditions {
				if newSignalRequest.Name == condition.EntrySignal.Name {
					condition.UpdateState(true)
					strategyAffected = true
				} else if newSignalRequest.Name == condition.ExitSignal.Name {
					condition.UpdateState(false)
					strategyAffected = true
				}
			}

			if strategyAffected {
				strategiesAffected += 1
			}
		}
	}

	return &eventmodels.NewSignalResult{
		RequestID:          newSignalRequest.RequestID,
		StrategiesAffected: strategiesAffected,
	}
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
			Conditions:      strategy.Conditions,
			OpenTradeLevels: openTradesByPriceLevel,
		})
	}

	return statsResult, nil
}
