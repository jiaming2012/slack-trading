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

type ExecResult struct {
	SuccessMsg string
	Err 	  error
}

func Exec(ctx context.Context, wg *sync.WaitGroup, symbol eventmodels.StockSymbol, optionsConfig eventmodels.OptionsConfigYAML, outDir, goEnv string) ExecResult {
	tradesAccountID := os.Getenv("TRADIER_TRADES_ACCOUNT_ID")
	tradierTradesOrderURL := fmt.Sprintf(os.Getenv("TRADIER_TRADES_URL_TEMPLATE"), tradesAccountID)
	brokerBearerToken := os.Getenv("TRADIER_BEARER_TOKEN")
	tradierTradesBearerToken := os.Getenv("TRADIER_TRADES_BEARER_TOKEN")
	eventStoreDbURL := os.Getenv("EVENTSTOREDB_URL")
	optionsExpirationURL := os.Getenv("OPTION_EXPIRATIONS_URL")
	optionChainURL := os.Getenv("OPTION_CHAIN_URL")

	optionConfig, err := optionsConfig.GetOption(symbol)
	if err != nil {
		return ExecResult{
			Err: fmt.Errorf("failed to get option config: %w", err),
		}
	}
	
	stockQuotesURL := os.Getenv("STOCK_QUOTES_URL")
	maxNoOfStrikes := optionConfig.MaxNoOfStrikes

	if optionConfig.MinDistanceBetweenStrikes == nil {
		return ExecResult{
			Err: fmt.Errorf("minDistanceBetweenStrikes is not set in config"),
		}
	}

	minDistanceBetweenStrikes := *optionConfig.MinDistanceBetweenStrikes
	expirationsInDays := optionConfig.ExpirationsInDays

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

	resultCh := make(chan ExecResult)

	go func(signalCh <-chan eventconsumers.SignalTriggeredEvent, optionsRequestExecutor *optionsapi.ReadOptionChainRequestExecutor, config eventmodels.OptionsConfigYAML, errCh chan ExecResult, DryRun bool) {
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			resultCh <- ExecResult{
				Err: fmt.Errorf("failed to load location: %w", err),
			}
			return
		}

		tradierOrderExecuter := eventmodels.NewTradierOrderExecuter(tradierTradesOrderURL, tradierTradesBearerToken, isDryRun)

		log.Infof("waiting for signal triggered events\n")

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

			// todo: separate this into a function
			// instead of finding the next friday, we can just use the expiration date from the config yaml
			nextOptionExpDate := deriveNextFriday(signal.Timestamp)
			data, err := services.FetchHistoricalOptionChainDataInput(&signal, signal.Timestamp, nextOptionExpDate, maxNoOfStrikes, minDistanceBetweenStrikes, expirationsInDays)

			if err != nil {
				log.Errorf("skipping event %v: failed to fetch option chain data: %v", signal, err)
				continue
			}

			if data == nil {
				log.Warnf("skipping event %v: failed to fetch option chain data", signal)
				continue
			}

			if len(data.OptionContracts) == 0 {
				log.Warnf("skipping event %v: no option chain data", signal)
				continue
			}

			go optionsRequestExecutor.ServeWithParams(ctx, readOptionChainReq, *data, true, signal.Timestamp, resultCh, errCh)

			// todo: metadata should be attached to each order
			// todo: this should be refactored to mostly use the same as eventmain
			highestEVBacktestOrder, err := services.DeriveHighestEVBacktesterOrder(ctx, resultCh, errCh, signal, tradierOrderExecuter, optionConfig, goEnv)
			if err != nil {
				log.Errorf("tradier executer: %v: send to market failed: %v", signal.Signal, err)
			}

			if highestEVBacktestOrder != nil {
				log.Infof("new trade: adding highest EV backtest order: %v", *highestEVBacktestOrder)
				allTrades = append(allTrades, highestEVBacktestOrder)
			}
		}

		if len(allTrades) == 0 {
			resultCh <- ExecResult{
				SuccessMsg: "no trades to process",
				Err:        nil,
			}
			return
		}

		candlesDTO, err := services.FetchCandlesFromBacktesterOrders(symbol, allTrades)
		if err != nil {
			resultCh <- ExecResult{
				Err: fmt.Errorf("failed to fetch candles: %w", err),
			}
			return
		}

		csvPath, err := services.ProcessBacktestTrades(symbol, allTrades, candlesDTO, outDir)
		if err != nil {
			resultCh <- ExecResult{
				Err: fmt.Errorf("ProcessBacktestTrades failed: %w", err),
			}
			return
		}

		resultCh <- ExecResult{
			SuccessMsg: fmt.Sprintf("processed %v backtest trades to %v", len(allTrades), csvPath),
			Err:        nil,
		}
	}(trackerV3OptionEVConsumer.GetSignalTriggeredCh(), optionChainRequestExector, optionsConfig, resultCh, isDryRun)

	trackerV3OptionEVConsumer.Replay(ctx)

	result := <-resultCh

	return result
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