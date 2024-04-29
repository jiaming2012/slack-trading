package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventconsumers"
	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	"slack-trading/src/eventpubsub"
	"slack-trading/src/eventservices"
	"slack-trading/src/utils"
)

func FetchCurrentStockAndOptionContracts(ctx context.Context, esdbProducer *eventproducers.EsdbProducer) ([]eventmodels.StockSymbol, []*eventmodels.OptionContract, error) {
	// todo: replace with a stream
	allOptionContracts, err := eventservices.FetchAll(ctx, esdbProducer.GetClient(), &eventmodels.OptionContract{})
	if err != nil {
		return []eventmodels.StockSymbol{}, nil, fmt.Errorf("failed to fetch all option contracts: %v", err)
	}

	// todo: replace with a stream
	allTrackers, err := eventservices.FetchAll(ctx, esdbProducer.GetClient(), &eventmodels.Tracker{})
	if err != nil {
		return []eventmodels.StockSymbol{}, nil, fmt.Errorf("failed to fetch all trackers: %v", err)
	}
	activeTrackers := eventservices.GetActiveTrackers(allTrackers)

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

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	// Set up
	utils.InitEnvironmentVariables()
	eventmodels.InitializeGlobalDispatcher()
	eventpubsub.Init()

	level, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(level)
	}

	log.Infof("Log level set to %v", log.GetLevel())

	stockQuotesURL := os.Getenv("STOCK_QUOTES_URL")
	calendarURL := os.Getenv("MARKET_CALENDAR_URL")
	optionChainURL := os.Getenv("OPTION_CHAIN_URL")
	brokerBearerToken := os.Getenv("TRADIER_BEARER_TOKEN")
	slackWebhookURL := os.Getenv("SLACK_WEBHOOK_URL")
	eventStoreDBURL := os.Getenv("EVENTSTOREDB_URL")
	accountID := os.Getenv("TRADIER_ACCOUNT_ID")
	tradierOrdersURL := fmt.Sprintf(os.Getenv("TRADIER_ORDERS_URL_TEMPLATE"), accountID)

	streamParams := []eventmodels.StreamParameter{
		// {StreamName: eventmodels.AccountsStreamName, Mutex: &sync.Mutex{}},
		// {StreamName: eventmodels.OptionAlertsStreamName, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.OptionChainTickStream, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.StockTickStream, Mutex: &sync.Mutex{}},
	}

	esdbProducer := eventproducers.NewESDBProducer(&wg, eventStoreDBURL, streamParams)
	esdbProducer.Start(ctx)
	eventconsumers.NewESDBConsumer(&wg, eventStoreDBURL).Start(ctx, eventmodels.OptionContractStream)
	eventconsumers.NewSlackNotifierClient(&wg, slackWebhookURL).Start(ctx)
	eventconsumers.NewTradierOrdersMonitoringWorker(&wg, tradierOrdersURL, brokerBearerToken).Start(ctx)

	currentStockSymbols, currentOptionContracts, err := FetchCurrentStockAndOptionContracts(ctx, esdbProducer)
	if err != nil {
		log.Fatalf("failed to fetch current option contracts: %v", err)
	}

	eventconsumers.NewOptionChainTickWriterWorker(&wg, stockQuotesURL, optionChainURL, brokerBearerToken, calendarURL).Start(ctx, currentStockSymbols, currentOptionContracts)

	log.Info("Main: init complete")

	// Wait for event clients to shut down
	wg.Wait()

	// EntrySignal -> shut down event clients
	cancel()

	log.Info("Main: gracefully stopped!")
}
