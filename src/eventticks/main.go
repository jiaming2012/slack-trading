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
	"slack-trading/src/utils"
)

func FetchCurrentOptionContracts() []eventmodels.OptionContract {
	return []eventmodels.OptionContract{}
}

func main() {
	ctx := context.Background()
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

	currentOptionContracts := FetchCurrentOptionContracts()
	eventconsumers.NewOptionChainTickWriterWorker(&wg, stockQuotesURL, optionChainURL, brokerBearerToken, calendarURL).Start(ctx, currentOptionContracts)

	streamParams := []eventmodels.StreamParameter{
		// {StreamName: eventmodels.AccountsStreamName, Mutex: &sync.Mutex{}},
		// {StreamName: eventmodels.OptionAlertsStreamName, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.OptionChainTickStream, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.StockTickStream, Mutex: &sync.Mutex{}},
	}

	eventproducers.NewESDBProducer(&wg, eventStoreDBURL, streamParams).Start(ctx)
	eventconsumers.NewESDBConsumer(&wg, eventStoreDBURL).Start(ctx, eventmodels.OptionContractStream)
	eventconsumers.NewSlackNotifierClient(&wg, slackWebhookURL).Start(ctx)
	eventconsumers.NewTradierOrdersMonitoringWorker(&wg, tradierOrdersURL, brokerBearerToken).Start(ctx)
}
