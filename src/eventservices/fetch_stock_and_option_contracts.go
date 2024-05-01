package eventservices

import (
	"context"
	"fmt"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"

	"slack-trading/src/eventmodels"
)

func FetchCurrentStockAndOptionContracts(ctx context.Context, esdbClient *esdb.Client) ([]eventmodels.StockSymbol, []*eventmodels.OptionContract, error) {
	// todo: replace with a stream
	allOptionContracts, err := FetchAll(ctx, esdbClient, &eventmodels.OptionContract{}, 0)
	if err != nil {
		return []eventmodels.StockSymbol{}, nil, fmt.Errorf("failed to fetch all option contracts: %v", err)
	}

	// todo: replace with a stream
	allTrackers, err := FetchAll(ctx, esdbClient, &eventmodels.Tracker{}, 0)
	if err != nil {
		return []eventmodels.StockSymbol{}, nil, fmt.Errorf("failed to fetch all trackers: %v", err)
	}
	activeTrackers := GetActiveTrackers(allTrackers)

	stockSymbolsMap := make(map[eventmodels.StockSymbol]struct{})
	optionContractsMap := make(map[eventmodels.EventStreamID]*eventmodels.OptionContract)
	for _, tracker := range activeTrackers {
		for _, optionContractID := range tracker.StartTracker.OptionContractIDs {
			contract := allOptionContracts[optionContractID]
			stockSymbolsMap[contract.UnderlyingSymbol] = struct{}{}
			optionContractsMap[optionContractID] = contract
		}
	}

	stockSymbols := make([]eventmodels.StockSymbol, 0, len(stockSymbolsMap))
	for stockSymbol := range stockSymbolsMap {
		stockSymbols = append(stockSymbols, stockSymbol)
	}

	optionContracts := make([]*eventmodels.OptionContract, 0, len(optionContractsMap))
	for _, optionContract := range optionContractsMap {
		optionContracts = append(optionContracts, optionContract)
	}

	return stockSymbols, optionContracts, nil
}
