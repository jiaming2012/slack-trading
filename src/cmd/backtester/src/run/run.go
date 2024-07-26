package run

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/cmd/backtester/src/services"
	"github.com/jiaming2012/slack-trading/src/eventconsumers"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventproducers/optionsapi"
)

func Exec(ctx context.Context, wg *sync.WaitGroup, optionsConfig eventmodels.OptionsConfigYAML, outDir, goEnv string) {
	tradesAccountID := os.Getenv("TRADIER_TRADES_ACCOUNT_ID")
	tradierTradesOrderURL := fmt.Sprintf(os.Getenv("TRADIER_TRADES_URL_TEMPLATE"), tradesAccountID)
	brokerBearerToken := os.Getenv("TRADIER_BEARER_TOKEN")
	tradierTradesBearerToken := os.Getenv("TRADIER_TRADES_BEARER_TOKEN")
	eventStoreDbURL := os.Getenv("EVENTSTOREDB_URL")
	optionsExpirationURL := os.Getenv("OPTION_EXPIRATIONS_URL")
	optionChainURL := os.Getenv("OPTION_CHAIN_URL")
	stockQuotesURL := os.Getenv("STOCK_QUOTES_URL")
	maxNoOfStrikes := 4
	minDistanceBetweenStrikes := 10.0
	expirationInDays := []int{7}

	fmt.Println("esdb url: ", eventStoreDbURL)

	isDryRun := strings.ToLower(os.Getenv("DRY_RUN")) == "true"

	symbol := eventmodels.StockSymbol("NVDA")
	streamName := eventmodels.StreamName(fmt.Sprintf("backtest-signals-%s", symbol))
	trackersClientV3 := eventconsumers.NewESDBConsumerStreamV2(wg, eventStoreDbURL, &eventmodels.TrackerV3{}, streamName)
	trackerV3OptionEVConsumer := eventconsumers.NewTrackerConsumerV3(trackersClientV3)

	optionChainRequestExector := &optionsapi.ReadOptionChainRequestExecutor{
		OptionsByExpirationURL: optionsExpirationURL,
		OptionChainURL:         optionChainURL,
		StockURL:               stockQuotesURL,
		BearerToken:            brokerBearerToken,
		GoEnv:                  goEnv,
	}

	wg.Add(1)

	go func(eventCh <-chan eventconsumers.SignalTriggeredEvent, optionsRequestExecutor *optionsapi.ReadOptionChainRequestExecutor, config eventmodels.OptionsConfigYAML, isDryRun bool) {
		defer wg.Done()

		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			log.Panicf("failed to load location: %v", err)
		}

		tradierOrderExecuter := eventmodels.NewTradierOrderExecuter(tradierTradesOrderURL, tradierTradesBearerToken, isDryRun)

		fmt.Printf("waiting for signal triggered events\n")

		var allTrades []*eventmodels.BacktesterOrder
		for event := range eventCh {
			log.Infof("received signal triggered event: %v", event.Signal)

			readOptionChainReq, err := eventconsumers.ProcessSignalTriggeredEvent(event, tradierOrderExecuter, optionsRequestExecutor, config, loc, goEnv)
			if err != nil {
				log.Errorf("failed to process signal triggered event: %v", err)
				continue
			}

			resultCh := make(chan map[string]interface{})
			errCh := make(chan error)

			nextOptionExpDate := deriveNextFriday(event.Timestamp)
			data, err := services.FetchHistoricalOptionChainDataInput(&event, event.Timestamp, nextOptionExpDate, maxNoOfStrikes, minDistanceBetweenStrikes, expirationInDays)

			if data == nil {
				log.Warnf("skipping event %v: failed to fetch option chain data", event)
				continue
			}

			go optionsRequestExecutor.ServeWithParams(ctx, readOptionChainReq, *data, true, event.Timestamp, resultCh, errCh)

			highestEVBacktestOrder, err := services.DeriveHighestEVBacktesterOrder(ctx, resultCh, errCh, event, tradierOrderExecuter, goEnv)
			if err != nil {
				log.Errorf("tradier executer: %v: send to market failed: %v", event.Signal, err)
			}

			if highestEVBacktestOrder != nil {
				log.Infof("new trade: adding highest EV backtest order: %v", *highestEVBacktestOrder)
				allTrades = append(allTrades, highestEVBacktestOrder)
			}
		}

		if len(allTrades) == 0 {
			log.Infof("no trades to process")
			return
		}

		candlesDTO, err := services.FetchCandlesFromBacktesterOrders(symbol, allTrades)
		if err != nil {
			log.Fatalf("failed to fetch candles: %v", err)
		}

		if err := services.ProcessBacktestTrades(symbol, allTrades, candlesDTO, outDir); err != nil {
			log.Errorf("ProcessBacktestTrades failed: %v", err)
		}

		log.Infof("analysis output: %v", outDir)
	}(trackerV3OptionEVConsumer.GetSignalTriggeredCh(), optionChainRequestExector, optionsConfig, isDryRun)

	trackerV3OptionEVConsumer.Replay(ctx)
}

func deriveNextFriday(now time.Time) time.Time {
	// find the next friday
	for {
		if now.Weekday() == time.Friday {
			break
		}

		now = now.AddDate(0, 0, 1)
	}

	return now
}