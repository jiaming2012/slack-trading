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

func Exec(ctx context.Context, wg *sync.WaitGroup, symbol eventmodels.StockSymbol, optionsConfig eventmodels.OptionsConfigYAML, outDir, goEnv string) {
	tradesAccountID := os.Getenv("TRADIER_TRADES_ACCOUNT_ID")
	tradierTradesOrderURL := fmt.Sprintf(os.Getenv("TRADIER_TRADES_URL_TEMPLATE"), tradesAccountID)
	brokerBearerToken := os.Getenv("TRADIER_BEARER_TOKEN")
	tradierTradesBearerToken := os.Getenv("TRADIER_TRADES_BEARER_TOKEN")
	eventStoreDbURL := os.Getenv("EVENTSTOREDB_URL")
	optionsExpirationURL := os.Getenv("OPTION_EXPIRATIONS_URL")
	optionChainURL := os.Getenv("OPTION_CHAIN_URL")

	optionConfig, err := optionsConfig.GetOption(symbol)
	if err != nil {
		log.Fatalf("failed to get option config: %v", err)
	}
	
	stockQuotesURL := os.Getenv("STOCK_QUOTES_URL")
	maxNoOfStrikes := 4
	minDistanceBetweenStrikes := 10.0
	expirationInDays := []int{7}

	log.Infof("esdb url: %v", eventStoreDbURL)

	isDryRun := strings.ToLower(os.Getenv("DRY_RUN")) == "true"

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

	go func(signalCh <-chan eventconsumers.SignalTriggeredEvent, optionsRequestExecutor *optionsapi.ReadOptionChainRequestExecutor, config eventmodels.OptionsConfigYAML, isDryRun bool) {
		defer wg.Done()

		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			log.Panicf("failed to load location: %v", err)
		}

		tradierOrderExecuter := eventmodels.NewTradierOrderExecuter(tradierTradesOrderURL, tradierTradesBearerToken, isDryRun)

		fmt.Printf("waiting for signal triggered events\n")

		var allTrades []*eventmodels.BacktesterOrder
		for signal := range signalCh {
			log.Infof("received signal triggered event: %v", signal.Signal)

			readOptionChainReq, err := eventconsumers.ProcessSignalTriggeredEvent(signal, tradierOrderExecuter, optionsRequestExecutor, config, loc, goEnv)
			if err != nil {
				log.Errorf("failed to process signal triggered event: %v", err)
				continue
			}

			resultCh := make(chan map[string]interface{})
			errCh := make(chan error)

			nextOptionExpDate := deriveNextFriday(signal.Timestamp)
			data, err := services.FetchHistoricalOptionChainDataInput(&signal, signal.Timestamp, nextOptionExpDate, maxNoOfStrikes, minDistanceBetweenStrikes, expirationInDays)

			if err != nil {
				log.Errorf("failed to fetch option chain data: %v", err)
			}

			if data == nil {
				log.Warnf("skipping event %v: failed to fetch option chain data", signal)
				continue
			}

			go optionsRequestExecutor.ServeWithParams(ctx, readOptionChainReq, *data, true, signal.Timestamp, resultCh, errCh)

			highestEVBacktestOrder, err := services.DeriveHighestEVBacktesterOrder(ctx, resultCh, errCh, signal, tradierOrderExecuter, goEnv)
			if err != nil {
				log.Errorf("tradier executer: %v: send to market failed: %v", signal.Signal, err)
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
			log.Fatalf("ProcessBacktestTrades failed: %v", err)
		}

		log.Infof("processed %v backtest trades at %v", len(allTrades), outDir)
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