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
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

type ExecResult struct {
	SuccessMsg string
	Err        error
}

func positionFetcher() ([]eventmodels.TradierPositionDTO, error) {
	panic("not implemented")
}

func Exec_Backtesterfunc(ctx context.Context, signalCh <-chan eventmodels.SignalTriggeredEvent, optionsRequestExecutor *eventmodels.ReadOptionChainRequestExecutor, projectsDir string, config BacktesterConfig, execResultCh chan ExecResult, apiKey string, isDryRun bool) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		execResultCh <- ExecResult{
			Err: fmt.Errorf("failed to load location: %w", err),
		}
		return
	}

	tradierOrderExecuter := eventmodels.NewTradierOrderExecuter(config.TradierTradesOrderURL, config.TradierTradesBearerToken, isDryRun, positionFetcher)

	log.Infof("waiting for signal triggered events\n")

	var allTrades []*eventmodels.BacktesterOrder
	errCh := make(chan error)

	go func() {
		for err := range errCh {
			log.Errorf("error: %v", err)
		}
	}()

	for signal := range signalCh {
		maxNoOfStrikes := config.MaxNoOfStrikes
		minDistanceBetweenStrikes := config.MinDistanceBetweenStrikes
		expirationsInDays := config.ExpirationsInDays
		goEnv := config.GoEnv

		log.Infof("received signal triggered event: %v", signal.Signal)

		readOptionChainReq, err := eventconsumers.ProcessSignalTriggeredEvent(signal, tradierOrderExecuter, optionsRequestExecutor, config.OptionsYAML, loc, goEnv)
		if err != nil {
			log.Errorf("failed to process signal triggered event: %v", err)
			continue
		}

		// todo: separate this into a function
		// instead of finding the next friday, we can just use the expiration date from the config yaml
		nextOptionExpDate := utils.DeriveNextFriday(signal.Timestamp)
		// nextOptionExpDate := utils.DeriveNextExpiration(signal.Timestamp, config.OptionsYAML.ExpirationsInDays)

		isHistorical := true
		data, err := optionsRequestExecutor.OptionsDataFetcher.FetchOptionChainDataInput(signal.Symbol, isHistorical, signal.Timestamp, signal.Timestamp, nextOptionExpDate, maxNoOfStrikes, minDistanceBetweenStrikes, expirationsInDays)

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

		resultCh := make(chan map[string]interface{})

		go optionsRequestExecutor.ServeWithParams(ctx, readOptionChainReq, *data, true, projectsDir, signal.Timestamp, resultCh, errCh)

		// todo: metadata should be attached to each order
		// todo: this should be refactored to mostly use the same as eventmain
		highestEVBacktestOrders, err := services.DeriveHighestEVBacktesterOrder(ctx, resultCh, errCh, signal, tradierOrderExecuter, config.OptionsYAML, config.RiskProfileConstraint, goEnv)
		if err != nil {
			log.Errorf("tradier executer: %v: send to market failed: %v", signal.Signal, err)
		}

		for _, order := range highestEVBacktestOrders {
			log.Infof("new trade: adding highest EV backtest order: %v", *order)
			allTrades = append(allTrades, order)
		}
	}

	if len(allTrades) == 0 {
		execResultCh <- ExecResult{
			SuccessMsg: "no trades to process",
			Err:        nil,
		}
		return
	}

	candlesDTO, err := services.FetchCandlesFromBacktesterOrders(config.Symbol, allTrades, apiKey)
	if err != nil {
		execResultCh <- ExecResult{
			Err: fmt.Errorf("failed to fetch candles: %w", err),
		}
		return
	}

	csvPath, err := services.ProcessBacktestTrades(config.Symbol, allTrades, candlesDTO, config.OutDir)
	if err != nil {
		execResultCh <- ExecResult{
			Err: fmt.Errorf("ProcessBacktestTrades failed: %w", err),
		}
		return
	}

	execResultCh <- ExecResult{
		SuccessMsg: fmt.Sprintf("processed %v backtest trades to %v", len(allTrades), csvPath),
		Err:        nil,
	}
}

type BacktesterConfig struct {
	TradierTradesOrderURL     string
	TradierTradesBearerToken  string
	MaxNoOfStrikes            int
	MinDistanceBetweenStrikes float64
	ExpirationsInDays         []int
	OptionsYAML               *eventmodels.OptionYAML
	Symbol                    eventmodels.StockSymbol
	RiskProfileConstraint     *eventmodels.RiskProfileConstraint
	OutDir                    string
	GoEnv                     string
}

func Exec(ctx context.Context, wg *sync.WaitGroup, symbol eventmodels.StockSymbol, optionsConfig eventmodels.OptionsConfigYAML, startAtEventNumber uint64, outDir, projectsDir string, goEnv string) ExecResult {
	tradesAccountID := os.Getenv("TRADIER_TRADES_ACCOUNT_ID")
	brokerBearerToken := os.Getenv("TRADIER_BEARER_TOKEN")
	eventStoreDbURL := os.Getenv("EVENTSTOREDB_URL")
	optionsExpirationURL := os.Getenv("OPTION_EXPIRATIONS_URL")
	optionChainURL := os.Getenv("OPTION_CHAIN_URL")
	polygonAPIKey := os.Getenv("POLYGON_API_KEY")

	optionConfig, err := optionsConfig.GetOption(symbol)
	if err != nil {
		return ExecResult{
			Err: fmt.Errorf("failed to get option config: %w", err),
		}
	}

	stockQuotesURL := os.Getenv("STOCK_QUOTES_URL")

	if optionConfig.MinDistanceBetweenStrikes == nil {
		return ExecResult{
			Err: fmt.Errorf("minDistanceBetweenStrikes is not set in config"),
		}
	}

	riskProfileConstraint := eventmodels.NewRiskProfileConstraint()
	riskProfileConstraint.AddItem(0.2, 1000)
	riskProfileConstraint.AddItem(0.8, 1800)

	backtesterConfig := BacktesterConfig{
		TradierTradesOrderURL:     fmt.Sprintf(os.Getenv("TRADIER_TRADES_URL_TEMPLATE"), tradesAccountID),
		TradierTradesBearerToken:  os.Getenv("TRADIER_TRADES_BEARER_TOKEN"),
		MaxNoOfStrikes:            optionConfig.MaxNoOfStrikes,
		MinDistanceBetweenStrikes: *optionConfig.MinDistanceBetweenStrikes,
		ExpirationsInDays:         optionConfig.ExpirationsInDays,
		OptionsYAML:               optionConfig,
		Symbol:                    symbol,
		RiskProfileConstraint:     riskProfileConstraint,
		OutDir:                    outDir,
		GoEnv:                     goEnv,
	}

	log.Infof("esdb url: %v", eventStoreDbURL)

	isDryRun := strings.ToLower(os.Getenv("DRY_RUN")) == "true"

	streamName := eventmodels.StreamName(fmt.Sprintf("backtest-signals-%s", symbol))
	trackersClientV3 := eventconsumers.NewESDBConsumerStreamV2(wg, eventStoreDbURL, &eventmodels.TrackerV3{}, streamName)
	trackerV3OptionEVConsumer := eventconsumers.NewTrackerConsumerV3(trackersClientV3)

	polygonOptionsDataFetcher := eventservices.NewPolygonOptionsDataFetcher("https://api.polygon.io", polygonAPIKey)

	optionChainRequestExector := &eventmodels.ReadOptionChainRequestExecutor{
		OptionsByExpirationURL: optionsExpirationURL,
		OptionChainURL:         optionChainURL,
		StockURL:               stockQuotesURL,
		OptionsDataFetcher:     polygonOptionsDataFetcher,
		BearerToken:            brokerBearerToken,
		GoEnv:                  goEnv,
	}

	resultCh := make(chan ExecResult)

	go Exec_Backtesterfunc(ctx, trackerV3OptionEVConsumer.GetSignalTriggeredCh(), optionChainRequestExector, projectsDir, backtesterConfig, resultCh, polygonAPIKey, isDryRun)

	trackerV3OptionEVConsumer.Replay(ctx, startAtEventNumber)

	result := <-resultCh

	return result
}
