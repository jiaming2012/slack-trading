package eventservices

import (
	"context"

	"slack-trading/src/eventmodels"
)

func GetCurrentStockAndOptionContracts(ctx context.Context, allOptionContracts []*eventmodels.OptionContractV1, allTrackers []*eventmodels.TrackerV1) ([]eventmodels.StockSymbol, eventmodels.OptionContracts, error) {
	allOptionContractsMap := make(map[eventmodels.EventStreamID]*eventmodels.OptionContractV1)
	for _, contract := range allOptionContracts {
		allOptionContractsMap[contract.GetMetaData().GetEventStreamID()] = contract
	}

	allTrackersMap := make(map[eventmodels.EventStreamID]*eventmodels.TrackerV1)
	for _, tracker := range allTrackers {
		allTrackersMap[tracker.GetMetaData().GetEventStreamID()] = tracker
	}

	activeTrackers := GetActiveTrackers(allTrackersMap)

	stockSymbolsMap := make(map[eventmodels.StockSymbol]struct{})
	optionContractsMap := make(map[eventmodels.EventStreamID]*eventmodels.OptionContractV1)
	for _, tracker := range activeTrackers {
		for _, optionContractID := range tracker.StartTracker.OptionContractIDs {
			contract := allOptionContractsMap[optionContractID]
			stockSymbolsMap[contract.UnderlyingSymbol] = struct{}{}
			optionContractsMap[optionContractID] = contract
		}
	}

	stockSymbols := make([]eventmodels.StockSymbol, 0, len(stockSymbolsMap))
	for stockSymbol := range stockSymbolsMap {
		stockSymbols = append(stockSymbols, stockSymbol)
	}

	optionContracts := make([]*eventmodels.OptionContractV1, 0, len(optionContractsMap))
	for _, optionContract := range optionContractsMap {
		optionContracts = append(optionContracts, optionContract)
	}

	return stockSymbols, optionContracts, nil
}
