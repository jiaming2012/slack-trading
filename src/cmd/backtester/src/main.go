package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"

	"github.com/jiaming2012/slack-trading/src/eventconsumers"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventproducers/optionsapi"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func runTicks() {
	req := eventmodels.ThetaDataHistOptionOHLCRequest{
		Root:       "AAPL",
		Right:      eventmodels.OptionTypeCall,
		Expiration: time.Date(2023, time.November, 3, 0, 0, 0, 0, time.UTC),
		Strike:     170.0,
		StartDate:  time.Date(2023, time.November, 3, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2023, time.November, 3, 0, 0, 0, 0, time.UTC),
		Interval:   1 * time.Minute,
	}

	baseURL := "http://localhost:25510"
	resp, err := eventservices.FetchHistOptionOHLC(baseURL, req)
	if err != nil {
		panic(fmt.Errorf("failed to fetch option ohlc: %w", err))
	}

	candlesDTO, err := resp.ToHistOptionOhlcDTO()
	if err != nil {
		panic(fmt.Errorf("failed to convert response to dto: %w", err))
	}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(fmt.Errorf("failed to load location: %w", err))
	}

	candles, err := eventmodels.HistOptionOhlcDTOs(candlesDTO).ConvertToHistOptionOhlc(loc)
	if err != nil {
		panic(fmt.Errorf("failed to convert dto to candle: %w", err))
	}

	for i, candle := range candles {
		fmt.Printf("%d: %+v\n", i, candle)
	}
}

func SendHighestEVTradeToMarket(ctx context.Context, resultCh chan map[string]interface{}, errCh chan error, event eventconsumers.SignalTriggeredEvent, tradierOrderExecuter *eventmodels.TradierOrderExecuter, goEnv string) (*eventmodels.TradierTradeRequest, error) {
	tracer := otel.GetTracerProvider().Tracer("SendHighestEVTradeToMarket")
	ctx, span := tracer.Start(ctx, "SendHighestEVTradeToMarket")
	defer span.End()

	logger := log.WithContext(ctx)

	select {
	case result := <-resultCh:
		if result != nil {
			options, ok := result["options"].(map[string][]*eventmodels.OptionSpreadContractDTO)
			if !ok {
				return nil, fmt.Errorf("options not found in result")
			}

			if calls, ok := options["calls"]; ok {
				highestEVLongCallSpreads, highestEVShortCallSpreads := eventconsumers.FindHighestEVPerExpiration(calls)
				for _, spread := range highestEVLongCallSpreads {
					if spread != nil {
						logger.WithField("event", "signal").Infof("Ignoring long call: %v", spread)
					} else {
						logger.WithField("event", "signal").Infof("No Positive EV Long Call found")
					}
				}

				for _, spread := range highestEVShortCallSpreads {
					if spread != nil {
						requestedPrc := 0.0
						if spread.CreditReceived != nil {
							requestedPrc = *spread.CreditReceived
						}

						tag := utils.EncodeTag(event.Signal, spread.Stats.ExpectedProfitShort, requestedPrc)

						span.AddEvent("PlaceTradeSpread:Call", trace.WithAttributes(attribute.String("tag", tag)))
						return &eventmodels.TradierTradeRequest{
							Underlying: event.Symbol,
							BuyToOpen:  spread.LongOptionSymbol,
							SellToOpen: spread.ShortOptionSymbol,
							Quantity:   1,
							Tag:        tag,
						}, nil
					} else {
						logger.WithField("event", "signal").Infof("No Positive EV Short Call found")
					}
				}
			} else {
				return nil, fmt.Errorf("calls not found in result")
			}

			if puts, ok := options["puts"]; ok {
				highestEVLongPutSpreads, highestEVShortPutSpreads := eventconsumers.FindHighestEVPerExpiration(puts)
				for _, spread := range highestEVLongPutSpreads {
					if spread != nil {
						logger.WithField("event", "signal").Infof("Ignoring long put: %v", spread)
					} else {
						logger.WithField("event", "signal").Infof("No Positive EV Long Put found")
					}
				}

				for _, spread := range highestEVShortPutSpreads {
					if spread != nil {
						requestedPrc := 0.0
						if spread.CreditReceived != nil {
							requestedPrc = *spread.CreditReceived
						}

						tag := utils.EncodeTag(event.Signal, spread.Stats.ExpectedProfitShort, requestedPrc)

						span.AddEvent("PlaceTradeSpread:Put", trace.WithAttributes(attribute.String("tag", tag)))

						return &eventmodels.TradierTradeRequest{
							Underlying: event.Symbol,
							BuyToOpen:  spread.LongOptionSymbol,
							SellToOpen: spread.ShortOptionSymbol,
							Quantity:   1,
							Tag:        tag,
						}, nil
					} else {
						logger.WithField("event", "signal").Infof("No Positive EV Short Put found")
					}
				}
			} else {
				return nil, fmt.Errorf("puts not found in result")
			}
		}

	case err := <-errCh:
		return nil, fmt.Errorf("error: %v", err)
	}

	return nil, nil
}

func run(ctx context.Context, wg *sync.WaitGroup, optionsConfig eventmodels.OptionsConfigYAML, goEnv string) {
	tradesAccountID := os.Getenv("TRADIER_TRADES_ACCOUNT_ID")
	tradierTradesOrderURL := fmt.Sprintf(os.Getenv("TRADIER_TRADES_URL_TEMPLATE"), tradesAccountID)
	brokerBearerToken := os.Getenv("TRADIER_BEARER_TOKEN")
	tradierTradesBearerToken := os.Getenv("TRADIER_TRADES_BEARER_TOKEN")
	eventStoreDbURL := os.Getenv("EVENTSTOREDB_URL")
	optionsExpirationURL := os.Getenv("OPTION_EXPIRATIONS_URL")
	optionChainURL := os.Getenv("OPTION_CHAIN_URL")
	stockQuotesURL := os.Getenv("STOCK_QUOTES_URL")

	fmt.Println("esdb url: ", eventStoreDbURL)

	isDryRun := strings.ToLower(os.Getenv("DRY_RUN")) == "true"

	trackersClientV3 := eventconsumers.NewESDBConsumerStream(wg, eventStoreDbURL, &eventmodels.TrackerV3{})
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
		for event := range eventCh {
			fmt.Printf("received signal triggered event: %v\n", event.Signal)
			readOptionChainReq, err := eventconsumers.ProcessSignalTriggeredEvent(event, tradierOrderExecuter, optionsRequestExecutor, config, loc, goEnv)
			if err != nil {
				log.Errorf("failed to process signal triggered event: %v", err)
				continue
			}

			resultCh := make(chan map[string]interface{})
			errCh := make(chan error)

			readOptionChainReq.EV.Signal = event.Signal

			// TODO: fill in!!
			data := optionsapi.FetchOptionChainDataInput{}

			go optionsRequestExecutor.ServeWithParams(ctx, readOptionChainReq, data, true, resultCh, errCh)

			tradierTradeReq, err := SendHighestEVTradeToMarket(ctx, resultCh, errCh, event, tradierOrderExecuter, goEnv)
			if err != nil {
				log.Errorf("tradier executer: %v: send to market failed: %v", event.Signal, err)
			}

			if err := BacktestTradeRequest(tradierTradeReq); err != nil {
				log.Errorf("tradier executer: %v: process trade request failed: %v", event.Signal, err)
			}
			// if err := eventservices.PlaceTradeSpread(ctx, tradierOrderExecuter.Url, tradierOrderExecuter.BearerToken, event.Symbol, spread.LongOptionSymbol, spread.ShortOptionSymbol, 1, tag, tradierOrderExecuter.DryRun); err != nil {
			// 	return nil, fmt.Errorf("tradierOrderExecuter.PlaceTradeSpread Put:: error placing trade: %v", err)
			// }

			log.Infof("processed signal triggered event: %v", event.Signal)
		}
	}(trackerV3OptionEVConsumer.GetSignalTriggeredCh(), optionChainRequestExector, optionsConfig, isDryRun)

	trackerV3OptionEVConsumer.Start(ctx, true)
}

func main() {
	ctx := context.Background()
	wg := sync.WaitGroup{}
	goEnv := "development"

	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		log.Fatalf("missing PROJECTS_DIR environment variable")
	}

	if err := utils.InitEnvironmentVariables(projectsDir, goEnv); err != nil {
		log.Panic(err)
	}

	// Load config
	optionsConfigInDir := path.Join(projectsDir, "slack-trading", "src", "options-config.yaml")
	data, err := os.ReadFile(optionsConfigInDir)
	if err != nil {
		log.Fatalf("failed to read options config: %v", err)
	}

	var optionsConfig eventmodels.OptionsConfigYAML
	if err := yaml.Unmarshal(data, &optionsConfig); err != nil {
		log.Fatalf("failed to unmarshal options config: %v", err)
	}

	_ = ctx
	_ = wg
	// run(ctx, &wg, optionsConfig, goEnv)

	root := eventmodels.StockSymbol("SPY")
	at := time.Date(2024, 7, 12, 0, 0, 0, 0, time.UTC)
	exp := time.Date(2024, 7, 19, 0, 0, 0, 0, time.UTC)
	res := FetchOptionThetaBulkHistOptionOhlc(root, exp, at)

	fmt.Printf("res: %+v\n", res)
}

func FetchOptionThetaBulkHistOptionOhlc(root eventmodels.StockSymbol, contractExpiration time.Time, at time.Time) optionsapi.FetchOptionChainDataInput {

	return optionsapi.FetchOptionChainDataInput{}
}

func BacktestTradeRequest(req *eventmodels.TradierTradeRequest) error {
	log.Infof("backtested trade request: %+v", req)

	// utils.CalculateOptionOrderSpreadResult()

	return nil
}
