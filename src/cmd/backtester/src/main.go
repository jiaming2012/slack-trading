package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
		Right:      eventmodels.ThetaDataOptionTypeCall,
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
				highestEVLongCallSpreads, highestEVShortCallSpreads, err := eventconsumers.FindHighestEVPerExpiration(calls)
				if err != nil {
					return nil, fmt.Errorf("FindHighestEVPerExpiration: failed to find highest EV per expiration: %w", err)
				}

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
				highestEVLongPutSpreads, highestEVShortPutSpreads, err := eventconsumers.FindHighestEVPerExpiration(puts)
				if err != nil {
					return nil, fmt.Errorf("FindHighestEVPerExpiration: failed to find highest EV per expiration: %w", err)
				}
				
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

func deriveNextOptionExpirationDate(now time.Time) time.Time {
	// find the next friday
	for {
		if now.Weekday() == time.Friday {
			break
		}

		now = now.AddDate(0, 0, 1)
	}

	return now
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
			if event.Symbol == eventmodels.StockSymbol("spx") || event.Symbol == eventmodels.StockSymbol("ndx") {
				log.Infof("ignoring %v event", event.Symbol)
				continue
			}

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
			data, err := FetchOptionChainDataInput(event.Symbol, event.Timestamp, deriveNextOptionExpirationDate(event.Timestamp))
			if err != nil {
				log.Fatalf("failed to fetch option chain data: %v", err)
			}

			go optionsRequestExecutor.ServeWithParams(ctx, readOptionChainReq, *data, true, event.Timestamp, resultCh, errCh)

			tradierTradeReq, err := SendHighestEVTradeToMarket(ctx, resultCh, errCh, event, tradierOrderExecuter, goEnv)
			if err != nil {
				log.Errorf("tradier executer: %v: send to market failed: %v", event.Signal, err)
			}

			if err := BacktestTradeRequest(tradierTradeReq); err != nil {
				log.Errorf("tradier executer: %v: process trade request failed: %v", event.Signal, err)
			}

			fmt.Printf("Place trade spread: %+v\n", tradierTradeReq)
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

	run(ctx, &wg, optionsConfig, goEnv)

	// root := eventmodels.StockSymbol("SPY")
	// at := time.Date(2024, 7, 12, 0, 0, 0, 0, time.UTC)
	// exp := time.Date(2024, 7, 19, 0, 0, 0, 0, time.UTC)

}

func convertToTime(contracts []eventmodels.OptionContractV3) (map[time.Time][]eventmodels.OptionContractV3, error) {
	result := make(map[time.Time][]eventmodels.OptionContractV3)
	for _, c := range contracts {
		ts, err := time.Parse("2006-01-02", string(c.ExpirationDate))
		if err != nil {
			log.Fatalf("failed to parse expiration date: %v", err)
		}

		if _, ok := result[ts]; !ok {
			result[ts] = make([]eventmodels.OptionContractV3, 0)
		}

		result[ts] = append(result[ts], c)
	}

	return result, nil
}

func FetchOptionChainDataInput(root eventmodels.StockSymbol, at, exp time.Time) (*optionsapi.FetchOptionChainDataInput, error) {
	request := eventmodels.ThetaDataBulkHistOptionOHLCRequest{
		Root:       root,
		Expiration: exp,
		StartDate:  at,
		EndDate:    at,
		Interval:   1 * time.Minute,
	}

	baseURL := "http://localhost:25510"
	resp, err := fetchOptionThetaBulkHistOptionOhlc(baseURL, request)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch option ohlc: %w", err)
	}

	// dtos, err := resp.ToBulkHistOptionOhlcDTO()
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to convert response to dto: %w", err)
	// }

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("failed to load location: %w", err)
	}

	// for i, dto := range dtos {
	// 	candles, err := eventmodels.HistOptionOhlcDTOs(dto.Candles).ConvertToHistOptionOhlc(loc)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to convert dto to candle: %w", err)
	// 	}

	// 	for j, candle := range candles {
	// 		fmt.Printf("Candle %d: %+v\n", j, candle)
	// 	}
	// }

	optionSpreadPerc := 0.005
	contracts, optionTickByExpirationMap, err := resp.GetOptionContractsV3(loc, optionSpreadPerc)
	if err != nil {
		return nil, fmt.Errorf("failed to get option contracts: %w", err)
	}

	optionTypes := []eventmodels.OptionType{eventmodels.OptionTypeCall, eventmodels.OptionTypePut}
	// optionsContractByExpirationMap, err := contracts.ConvertToOptionContractsV3(root, optionTypes)
	// if err != nil {
	// 	return FetchOptionChainDataInput{}, fmt.Errorf("failed to convert Tradier options to contracts: %v", err)
	// }

	optionTickByExpirationTimeMap, err := convertToTime(contracts)
	if err != nil {
		return nil, fmt.Errorf("failed to convert expiration date to time: %w", err)
	}

	stockSpreadPerc := 0.001
	now := time.Date(2024, 7, 12, 13, 0, 0, 0, loc)
	closestStockTickDTO, err := findClosestStockTickItemDTO(request, now, loc, stockSpreadPerc)
	if err != nil {
		return nil, fmt.Errorf("failed to find closest stock tick: %w", err)
	}

	maxNoOfStrikes := 4
	minDistanceBetweenStrikes := 10.0
	expirationInDays := []int{7}
	_, filteredOptions := eventservices.FilterOptions(
		optionTickByExpirationTimeMap,
		closestStockTickDTO,
		expirationInDays,
		optionTypes,
		minDistanceBetweenStrikes,
		maxNoOfStrikes,
		at,
	)

	return &optionsapi.FetchOptionChainDataInput{
		StockTickItemDTO:               closestStockTickDTO,
		OptionChainTickByExpirationMap: optionTickByExpirationMap,
		FilteredOptions:                filteredOptions,
	}, nil
}

func findClosestStockTickItemDTO(req eventmodels.ThetaDataBulkHistOptionOHLCRequest, now time.Time, loc *time.Location, spreadPerc float64) (*eventmodels.StockTickItemDTO, error) {
	candlesNearPriceReversedDTO, err := eventservices.FetchFinancialModelingPrepChart(req.Root, "1min", now, now.AddDate(0, 0, 1))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch underlying price near close: %w", err)
	}

	candlesNearPriceDTO := utils.ReverseCandlesDTO(candlesNearPriceReversedDTO)

	var candles []*eventmodels.Candle
	for _, dto := range candlesNearPriceDTO {
		c, err := dto.ToCandle(loc)
		if err != nil {
			return nil, fmt.Errorf("failed to convert dto to candle: %w", err)
		}

		candles = append(candles, &c)
	}

	closestPrice, err := findClosestPriceBeforeOrAt(candles, now)
	if err != nil {
		return nil, fmt.Errorf("failed to find closest candle: %w", err)
	}

	// Find the closest candle before or at the current time
	fmt.Printf("candlesNearPrice: %+v\n", closestPrice)

	return &eventmodels.StockTickItemDTO{
		Symbol: string(req.Root),
		Bid:    closestPrice,
		Ask:    closestPrice * (1 + spreadPerc),
	}, nil
}

func findClosestPriceBeforeOrAt(candles []*eventmodels.Candle, at time.Time) (float64, error) {
	var closestCandle *eventmodels.Candle
	for _, candle := range candles {
		if candle.Timestamp.After(at) {
			break
		}

		closestCandle = candle
	}

	return closestCandle.Open, nil
}

func fetchOptionThetaBulkHistOptionOhlc(baseURL string, r eventmodels.ThetaDataBulkHistOptionOHLCRequest) (*eventmodels.ThetaDataBulkResponse, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	url := fmt.Sprintf("%s/v2/bulk_hist/option/ohlc", baseURL)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("FetchHistOptionOHLC: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("root", string(r.Root))
	q.Add("exp", r.Expiration.Format("20060102"))
	q.Add("start_date", r.StartDate.Format("20060102"))
	q.Add("end_date", r.EndDate.Format("20060102"))
	q.Add("ivl", fmt.Sprintf("%d", (int(r.Interval/time.Minute)*60000)))

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")

	log.Printf("FetchHistOptionOHLC: fetching option ohlc from %v", req.URL.String())

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FetchHistOptionOHLC: failed to fetch option ohlc: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FetchHistOptionOHLC: failed to fetch option ohlc, http code %v", res.Status)
	}

	var dto eventmodels.ThetaDataBulkResponse
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("FetchHistOptionOHLC: failed to decode json: %w", err)
	}

	return &dto, nil
}

func BacktestTradeRequest(req *eventmodels.TradierTradeRequest) error {
	log.Infof("backtested trade request: %+v", req)

	// utils.CalculateOptionOrderSpreadResult()

	return nil
}
