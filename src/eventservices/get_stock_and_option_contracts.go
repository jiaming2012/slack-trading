package eventservices

import (
	"context"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func GetCurrentStockAndOptionContracts(ctx context.Context, allOptionContracts []*eventmodels.OptionContractV1, allTrackers []*eventmodels.TrackerV3) ([]eventmodels.StockSymbol, eventmodels.OptionContracts, error) {
	allOptionContractsMap := make(map[eventmodels.OptionSymbol]*eventmodels.OptionContractV1)
	for _, contract := range allOptionContracts {
		allOptionContractsMap[contract.Symbol] = contract
	}

	allTrackersMap := make(map[eventmodels.EventStreamID]*eventmodels.TrackerV3)
	for _, tracker := range allTrackers {
		allTrackersMap[tracker.GetMetaData().GetEventStreamID()] = tracker
	}

	activeTrackers := GetActiveStockAndOptionTrackers(allTrackersMap)

	stockSymbolsMap := make(map[eventmodels.StockSymbol]struct{})
	optionContractsMap := make(map[eventmodels.OptionSymbol]*eventmodels.OptionContractV1)
	for _, tracker := range activeTrackers {
		for _, optionContractSymbol := range tracker.StartTracker.OptionContractSymbols {
			contract := allOptionContractsMap[optionContractSymbol]
			stockSymbolsMap[contract.UnderlyingSymbol] = struct{}{}
			optionContractsMap[optionContractSymbol] = contract
		}
	}

	stockSymbols := make([]eventmodels.StockSymbol, 0, len(stockSymbolsMap))
	for s := range stockSymbolsMap {
		stockSymbols = append(stockSymbols, s)
	}

	optionContracts := make([]*eventmodels.OptionContractV1, 0, len(optionContractsMap))
	for _, c := range optionContractsMap {
		optionContracts = append(optionContracts, c)
	}

	return stockSymbols, optionContracts, nil
}
