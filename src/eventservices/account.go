package eventservices

import (
	"fmt"
	"github.com/google/uuid"
	"slack-trading/src/eventmodels"
	"slack-trading/src/models"
)

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
			OpenTradeLevels: openTradesByPriceLevel,
		})
	}

	return statsResult, nil
}
