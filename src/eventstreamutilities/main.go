package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	"slack-trading/src/eventpubsub"
	"slack-trading/src/eventservices"
	"slack-trading/src/utils"
)

func getStreamSize(ctx context.Context, esdbClient *esdb.Client) {
	streamNames := eventservices.ListAllStreams(ctx, esdbClient)

	for _, streamName := range streamNames {
		size, err := eventservices.CalculateStreamSize(ctx, esdbClient, streamName)
		if err != nil {
			log.Errorf("Error calculating size for stream %s: %v", streamName, err)
			continue
		}

		sizeInMb := float64(size) / 1024 / 1024

		fmt.Printf("Stream: %s, Size: %.2f MB\n", streamName, sizeInMb)
	}
}

func GetEsdbClient(ctx context.Context, wg *sync.WaitGroup, eventStoreDBURL string) (*esdb.Client, error) {
	config, err := esdb.ParseConnectionString(eventStoreDBURL)
	if err != nil {
		log.Fatalf("Error parsing connection string: %v", err)
	}

	// Create a new client
	esdbCli, err := esdb.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	return esdbCli, nil
}

type BrokerCredentials struct {
	BearerToken          string
	StockQuotesURL       string
	OptionChainURL       string
	OptionsExpirationURL string
}

func main() {
	ctx := context.Background()
	wg := sync.WaitGroup{}

	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		panic("missing PROJECTS_DIR environment variable")
	}

	goEnv := os.Getenv("GO_ENV")
	if goEnv == "" {
		panic("missing GO_ENV environment variable")
	}

	// Set up
	utils.InitEnvironmentVariables(projectsDir, goEnv)
	eventmodels.InitializeGlobalDispatcher()
	eventpubsub.Init()

	// Environment variables
	level, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(level)
	}

	// Set the connection details
	eventStoreDBURL := os.Getenv("EVENTSTOREDB_URL")
	brokerCreds := BrokerCredentials{
		BearerToken:          os.Getenv("TRADIER_BEARER_TOKEN"),
		StockQuotesURL:       os.Getenv("STOCK_QUOTES_URL"),
		OptionChainURL:       os.Getenv("OPTION_CHAIN_URL"),
		OptionsExpirationURL: os.Getenv("OPTION_EXPIRATIONS_URL"),
	}

	// Log connection details
	if eventStoreDBURL == "" {
		log.Fatalf("EVENTSTOREDB_URL is required")
	} else {
		log.Infof("EventStoreDB URL: %s", eventStoreDBURL)
	}

	esdbConn, err := GetEsdbClient(ctx, &wg, eventStoreDBURL)
	if err != nil {
		log.Fatalf("failed to create ESDB client: %v", err)
	}

	defer esdbConn.Close()

	var commandStr string
	if len(os.Args) > 1 {
		commandStr = os.Args[1]
	}

	log.Debugf("command: %s", commandStr)

	var command int
	switch commandStr {
	case "LIST_ALL_STREAMS":
		command = 1
	case "CALCULATE_STREAM_SIZES":
		command = 2
	case "FETCH_AND_STORE_TRADIER_OPTIONS":
		command = 3
	case "START_TRACKING":
		command = 4
	case "START_TRACKING_FX":
		command = 5
	case "STOP_TRACKING":
		command = 6
	case "CREATE_SIGNAL":
		command = 7
	default:
		fmt.Printf("Enter a command:\n1. List all streams\n2. Calculate all stream sizes\n3. Fetch and store Tradier options\n4. Start tracking\n5. Start tracking FX\n6. Stop tracking\n7. Create signal\n")
		if err := utils.ReadLineFromStdin(&commandStr); err != nil {
			log.Fatalf("failed to read command: %v", err)
		}

		commandStr = strings.TrimSpace(commandStr)
		command, err = strconv.Atoi(commandStr)
		if err != nil {
			log.Fatalf("failed to convert command to int: %v", err)
		}

		fmt.Printf("***********************\n")
	}

	log.Infof("running command: %d", command)

	switch command {
	case 1:
		streams := eventservices.ListAllStreams(ctx, esdbConn)
		for _, stream := range streams {
			fmt.Println(stream)
		}
	case 2:
		getStreamSize(ctx, esdbConn)
		wg.Done()
	case 3:
		// Setup
		fxTickCh := make(chan *eventmodels.FxTick)
		esdbProducer := eventproducers.NewESDBProducer(&wg, eventStoreDBURL, []eventmodels.StreamParameter{})
		esdbProducer.Start(ctx, fxTickCh)

		params, err := getOptionParametersComponents(nil, "options")
		if err != nil {
			log.Fatalf("failed to get option parameters: %v", err)
		}

		existingOptionContracts, err := eventservices.FetchAllDeprecated(ctx, esdbConn, &eventmodels.OptionContractV1{})
		if err != nil {
			log.Fatalf("failed to fetch existing contracts: %v", err)
		}

		cache := make(map[eventmodels.OptionSymbol]*eventmodels.OptionContractV1)
		for _, contract := range existingOptionContracts {
			cache[contract.Symbol] = contract
		}

		requestID := uuid.New()

		if _, err = FetchAndStoreTradierOptions(ctx, &wg, esdbProducer, params, cache, eventStoreDBURL, brokerCreds, requestID); err != nil {
			log.Fatalf("failed to fetch and store Tradier options: %v", err)
		}

		wg.Done()
	case 4:
		existingOptionContracts, err := eventservices.FetchAllDeprecated(ctx, esdbConn, &eventmodels.OptionContractV1{})
		if err != nil {
			log.Fatalf("failed to fetch existing contracts: %v", err)
		}

		cache := make(map[eventmodels.OptionSymbol]*eventmodels.OptionContractV1)
		for _, contract := range existingOptionContracts {
			cache[contract.Symbol] = contract
		}

		// create a new tracker
		if err = StartTrackingStockAndOptions(ctx, &wg, cache, eventStoreDBURL, brokerCreds); err != nil {
			log.Fatalf("failed to start tracker: %v", err)
		}

		wg.Done()
	case 5:
		// create a new tracker
		if err = StartTrackingFx(ctx, &wg, eventStoreDBURL); err != nil {
			log.Fatalf("failed to start fx tracker: %v", err)
		}

		wg.Done()
	case 6:
		existingOptionContracts, err := eventservices.FetchAllDeprecated(ctx, esdbConn, &eventmodels.OptionContractV1{})
		if err != nil {
			log.Fatalf("failed to fetch existing contracts: %v", err)
		}

		cache := make(map[eventmodels.OptionSymbol]*eventmodels.OptionContractV1)
		for _, contract := range existingOptionContracts {
			cache[contract.Symbol] = contract
		}

		if err = StopTracking(ctx, &wg, cache, eventStoreDBURL, brokerCreds); err != nil {
			log.Fatalf("failed to stop tracker: %v", err)
		}

		wg.Done()
	case 7:
		if err = CreateSignal(ctx, &wg, eventStoreDBURL); err != nil {
			log.Fatalf("failed to create signal: %v", err)
		}

		wg.Done()
	default:
		log.Fatalf("Invalid command: %d", command)
	}

	wg.Wait()
}

func StopTracking(ctx context.Context, wg *sync.WaitGroup, optionContractsCache map[eventmodels.OptionSymbol]*eventmodels.OptionContractV1, eventStoreDBURL string, brokerCreds BrokerCredentials) error {
	// Setup
	fxTickCh := make(chan *eventmodels.FxTick)
	esdbProducer := eventproducers.NewESDBProducer(wg, eventStoreDBURL, []eventmodels.StreamParameter{})
	esdbProducer.Start(ctx, fxTickCh)

	// Get symbol
	var symbol eventmodels.StockSymbol
	if len(os.Args) > 2 {
		symbol = eventmodels.StockSymbol(os.Args[2])
	}

	if symbol == "" {
		var s string
		fmt.Printf("Enter an underlying symbol (e.g. coin): ")
		if err := utils.ReadLineFromStdin(&s); err != nil {
			return fmt.Errorf("failed to read symbol: %v", err)
		}

		symbol = eventmodels.StockSymbol(s)
	}

	// Get reason
	var reason string
	if len(os.Args) > 3 {
		reason = os.Args[3]
	}

	if reason == "" {
		fmt.Printf("Enter a reason: ")
		if err := utils.ReadLineFromStdin(&reason); err != nil {
			return fmt.Errorf("failed to read reason: %v", err)
		}
	}

	// Check
	allTrackers, err := eventservices.FetchAllDeprecated(ctx, esdbProducer.GetClient(), &eventmodels.TrackerV3{})
	if err != nil {
		return fmt.Errorf("failed to fetch all trackers: %v", err)
	}

	stopTracking := make([]*eventmodels.TrackerV3, 0)

	activeTrackers := eventservices.GetActiveStockAndOptionTrackers(allTrackers)
	for _, t := range activeTrackers {
		if t.StartTracker.UnderlyingSymbol == symbol && t.StartTracker.Reason == reason {
			stopTracking = append(stopTracking, t)
		}
	}

	if len(stopTracking) == 0 {
		return fmt.Errorf("no active trackers found for symbol: %s", symbol)
	}

	requestID := uuid.New()

	log.Infof("stop tracking symbol: %s, reason: %s, requestID: %s", symbol, reason, requestID.String())

	for _, startTracker := range stopTracking {
		now := time.Now().UTC()

		tracker := eventmodels.NewStopTracker(startTracker.Meta.GetEventStreamID(), now, reason, requestID)

		// Save the tracker
		if err := esdbProducer.Save(tracker); err != nil {
			return fmt.Errorf("failed to save tracker: %v", err)
		}
	}

	return nil
}

func CreateSignal(ctx context.Context, wg *sync.WaitGroup, eventStoreDBURL string) error {
	// Setup
	fxTickCh := make(chan *eventmodels.FxTick)
	esdbProducer := eventproducers.NewESDBProducer(wg, eventStoreDBURL, []eventmodels.StreamParameter{})
	esdbProducer.Start(ctx, fxTickCh)

	allTrackers, err := eventservices.FetchAllDeprecated(ctx, esdbProducer.GetClient(), &eventmodels.TrackerV3{})
	if err != nil {
		return fmt.Errorf("failed to fetch all trackers: %v", err)
	}

	// Get symbol
	var symbol eventmodels.StockSymbol
	if len(os.Args) > 2 {
		symbol = eventmodels.StockSymbol(os.Args[2])
	}

	if symbol == "" {
		var s string
		fmt.Printf("Enter an underlying symbol (e.g. coin): ")
		if err := utils.ReadLineFromStdin(&s); err != nil {
			return fmt.Errorf("failed to read symbol: %v", err)
		}

		symbol = eventmodels.StockSymbol(s)
	}

	// Check:
	activeTrackers := eventservices.GetActiveStockAndOptionTrackers(allTrackers)
	found := false
	for _, t := range activeTrackers {
		if symbol == t.StartTracker.UnderlyingSymbol {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("no active trackers found for symbol: %s", symbol)
	}

	// Get signal name
	var signalName string
	if len(os.Args) > 3 {
		signalName = os.Args[3]
	}

	if signalName == "" {
		fmt.Printf("Enter a signal name: ")
		if err := utils.ReadLineFromStdin(&signalName); err != nil {
			return fmt.Errorf("failed to read signal name: %v", err)
		}
	}

	// Get timeframe
	var timeframeStr string
	if len(os.Args) > 4 {
		timeframeStr = os.Args[4]
	}

	if timeframeStr == "" {
		fmt.Printf("Enter a timeframe: ")
		if err := utils.ReadLineFromStdin(&timeframeStr); err != nil {
			return fmt.Errorf("failed to read timeframe: %v", err)
		}
	}

	timeframe, err := strconv.Atoi(timeframeStr)
	if err != nil {
		return fmt.Errorf("failed to convert timeframe to int: %v", err)
	}

	// Create signal tracker
	requestID := uuid.New()

	log.Infof("create signal requestID: %s", requestID.String())

	header := eventmodels.SignalRequestHeader{
		Symbol:    symbol,
		Source:    eventmodels.SignalSourceManual,
		Timeframe: uint(timeframe),
	}
	ts := time.Now().UTC()
	signalTracker := eventmodels.NewSignalTrackerV2(signalName, header, ts, requestID)

	// Save the signal tracker
	if err := esdbProducer.Save(signalTracker); err != nil {
		return fmt.Errorf("failed to save signal tracker: %v", err)
	}

	return nil
}

func StartTrackingFx(ctx context.Context, wg *sync.WaitGroup, eventStoreDBURL string) error {
	// Setup
	fxTickCh := make(chan *eventmodels.FxTick)
	esdbProducer := eventproducers.NewESDBProducer(wg, eventStoreDBURL, []eventmodels.StreamParameter{})
	esdbProducer.Start(ctx, fxTickCh)

	allTrackersMap, err := eventservices.FetchAllDeprecated(ctx, esdbProducer.GetClient(), &eventmodels.TrackerV3{})
	if err != nil {
		return fmt.Errorf("failed to fetch all trackers: %v", err)
	}

	var allTrackers []*eventmodels.TrackerV3
	for _, t := range allTrackersMap {
		allTrackers = append(allTrackers, t)
	}

	// Check:
	activeTrackers := eventservices.GetActiveFxTrackers(allTrackers)

	// Check if tracker already exists
	params, err := getOptionParametersComponents(activeTrackers, "fx")
	if err != nil {
		return fmt.Errorf("failed to get option parameters: %v", err)
	}

	// Create tracker
	requestID := uuid.New()

	log.Infof("start tracking symbol: %s, requestID: %s", params.StockSymbol, requestID.String())

	symbol := params.FxSymbol
	now := time.Now().UTC()

	tracker := eventmodels.NewStartFxTracker(symbol, now, params.Reason, requestID)

	// Save the tracker
	if err := esdbProducer.Save(tracker); err != nil {
		return fmt.Errorf("failed to save tracker: %v", err)
	}

	return nil
}

func StartTrackingStockAndOptions(ctx context.Context, wg *sync.WaitGroup, optionContractsCache map[eventmodels.OptionSymbol]*eventmodels.OptionContractV1, eventStoreDBURL string, brokerCreds BrokerCredentials) error {
	// Setup
	fxTickCh := make(chan *eventmodels.FxTick)
	esdbProducer := eventproducers.NewESDBProducer(wg, eventStoreDBURL, []eventmodels.StreamParameter{})
	esdbProducer.Start(ctx, fxTickCh)

	allTrackers, err := eventservices.FetchAllDeprecated(ctx, esdbProducer.GetClient(), &eventmodels.TrackerV3{})
	if err != nil {
		return fmt.Errorf("failed to fetch all trackers: %v", err)
	}

	// Check:
	activeTrackers := eventservices.GetActiveStockAndOptionTrackers(allTrackers)

	// Check if tracker already exists
	params, err := getOptionParametersComponents(activeTrackers, "options")
	if err != nil {
		return fmt.Errorf("failed to get option parameters: %v", err)
	}

	// Create tracker
	requestID := uuid.New()

	log.Infof("start tracking symbol: %s, requestID: %s", params.StockSymbol, requestID.String())

	options, err := FetchAndStoreTradierOptions(ctx, wg, esdbProducer, params, optionContractsCache, eventStoreDBURL, brokerCreds, requestID)
	if err != nil {
		return fmt.Errorf("failed to fetch and store Tradier options: %v", err)
	}

	optionContractSymbols := make([]eventmodels.OptionSymbol, 0)
	for _, option := range options {
		optionContractSymbols = append(optionContractSymbols, option.Symbol)
	}

	underlyingSymbol := params.StockSymbol
	now := time.Now().UTC()

	tracker := eventmodels.NewStartTracker(underlyingSymbol, optionContractSymbols, now, params.Reason, requestID)

	// Save the tracker
	if err := esdbProducer.Save(tracker); err != nil {
		return fmt.Errorf("failed to save tracker: %v", err)
	}

	return nil
}

func FetchAndStoreTradierOptions(ctx context.Context, wg *sync.WaitGroup, esdbProducer *eventproducers.EsdbProducer, params eventmodels.OptionParameterComponents, optionContractsCache map[eventmodels.OptionSymbol]*eventmodels.OptionContractV1, eventStoreDBURL string, brokerCreds BrokerCredentials, requestID uuid.UUID) ([]*eventmodels.OptionContractV1, error) {
	log.Infof("fetching options for symbol: %s, requestID: %s", params.StockSymbol, requestID.String())

	optionTypes := []eventmodels.OptionType{eventmodels.Call, eventmodels.Put}

	options, err := eventservices.FetchOptionChainWithParamsV1(requestID, brokerCreds.OptionsExpirationURL, brokerCreds.OptionChainURL, brokerCreds.StockQuotesURL, brokerCreds.BearerToken, params.StockSymbol, optionTypes, params.ExpirationInDays, params.MinDistanceBetweenStrikes, params.MaxNoOfStrikes)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch option chain: %v", err)
	}

	log.Infof("Saving %d option contracts ...\n", len(options))

	created := make([]*eventmodels.OptionContractV1, 0)
	for i := 0; i < len(options); i++ {
		option := options[i]

		if o, found := optionContractsCache[option.Symbol]; found {
			created = append(created, o)
			log.Debugf("skip save: option contract %v already exists", option.Symbol)
			continue
		}

		esdbProducer.Save(&option)
		created = append(created, &option)

		log.Infof("Created contract -> Expiration: %s, Type: %s, Strike: %.2f\n", option.Expiration.Format("2006-01-02"), option.OptionType, option.Strike)
	}

	log.Info("Done")

	return created, nil
}

func getOptionParametersComponents(activeTrackers map[eventmodels.EventStreamID]*eventmodels.TrackerV3, mode string) (eventmodels.OptionParameterComponents, error) {
	var reason string
	var err error
	var symbol eventmodels.StockSymbol

	if activeTrackers != nil {
		// Get the reason
		if mode == "fx" {
			var fxSymbol eventmodels.FxSymbol
			// Get the underlying fx symbol
			if len(os.Args) > 2 {
				fxSymbol = eventmodels.FxSymbol(os.Args[2])
			}

			if fxSymbol == "" {
				var quote string
				fmt.Printf("Enter an fx quote symbol (e.g. EUR): ")
				if err := utils.ReadLineFromStdin(&quote); err != nil {
					return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to read symbol: %v", err)
				}

				var base string
				fmt.Printf("Enter an fx base symbol (e.g. USD): ")
				if err := utils.ReadLineFromStdin(&base); err != nil {
					return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to read symbol: %v", err)
				}

				fxSymbol = eventmodels.FxSymbol(fmt.Sprintf("%s_%s", quote, base))
			}

			// Get the reason
			if len(os.Args) > 3 {
				reason = os.Args[3]
			}

			if reason == "" {
				fmt.Printf("Enter a reason: ")
				if err = utils.ReadLineFromStdin(&reason); err != nil {
					return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to read reason: %v", err)
				}
			}

			return eventmodels.OptionParameterComponents{
				FxSymbol: fxSymbol,
				Reason:   reason,
			}, nil
		} else {
			// Get the underlying stock symbol
			if len(os.Args) > 2 {
				symbol = eventmodels.StockSymbol(os.Args[2])
			}

			if symbol == "" {
				var s string
				fmt.Printf("Enter an underlying symbol (e.g. coin): ")
				if err := utils.ReadLineFromStdin(&s); err != nil {
					return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to read symbol: %v", err)
				}

				symbol = eventmodels.StockSymbol(s)
			}

			// Get the reason
			if len(os.Args) > 6 {
				reason = os.Args[6]
			}

			if reason == "" {
				fmt.Printf("Enter a reason: ")
				if err = utils.ReadLineFromStdin(&reason); err != nil {
					return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to read reason: %v", err)
				}
			}

			for _, tracker := range activeTrackers {
				if tracker.StartTracker.UnderlyingSymbol == symbol && tracker.StartTracker.Reason == reason {
					return eventmodels.OptionParameterComponents{}, fmt.Errorf("tracker already exists for symbol %s and reason %s", symbol, reason)
				}
			}
		}
	}

	// Get the underlying stock symbol
	if len(os.Args) > 2 {
		symbol = eventmodels.StockSymbol(os.Args[2])
	}

	if symbol == "" {
		var s string
		fmt.Printf("Enter an underlying symbol (e.g. coin): ")
		if err := utils.ReadLineFromStdin(&s); err != nil {
			return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to read symbol: %v", err)
		}

		symbol = eventmodels.StockSymbol(s)
	}

	// Get expiration in days
	var expirationInDays []int
	if len(os.Args) > 3 {
		expirationInDays, err = utils.AtoiSlice(os.Args[3])
		if err != nil {
			return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to parse expiration in days: %v", err)
		}
	}

	if len(expirationInDays) == 0 {
		fmt.Printf("Enter expiration in days (comma-separated list, e.g. 7, 14, 21): ")

		var expirationInDaysStr string
		if err = utils.ReadLineFromStdin(&expirationInDaysStr); err != nil {
			return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to read expiration in days: %v", err)
		}

		// parse the input
		expirationInDays, err = utils.AtoiSlice(expirationInDaysStr)
		if err != nil {
			return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to parse expiration in days: %v", err)
		}
	}

	// Get min distance between strikes
	var minDistanceBetweenStrikes float64
	if len(os.Args) > 4 {
		minDistanceBetweenStrikes, err = strconv.ParseFloat(os.Args[4], 64)
		if err != nil {
			return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to parse min distance between strikes: %v", err)
		}
	}

	if minDistanceBetweenStrikes == 0 {
		var s string
		fmt.Printf("Enter min distance between strikes (e.g. 10.0): ")
		if err = utils.ReadLineFromStdin(&s); err != nil {
			return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to read min distance between strikes: %v", err)
		}

		minDistanceBetweenStrikes, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to parse min distance between strikes: %v", err)
		}
	}

	// Get max number of strikes
	var maxNoOfStrikes int
	if len(os.Args) > 5 {
		maxNoOfStrikes, err = strconv.Atoi(os.Args[5])
		if err != nil {
			return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to parse max number of strikes: %v", err)
		}
	}

	if maxNoOfStrikes == 0 {
		var s string
		fmt.Printf("Enter max number of strikes (e.g. 5): ")
		if err = utils.ReadLineFromStdin(&s); err != nil {
			return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to read max number of strikes: %v", err)
		}

		maxNoOfStrikes, err = strconv.Atoi(s)
		if err != nil {
			return eventmodels.OptionParameterComponents{}, fmt.Errorf("getOptionParameters:failed to parse max number of strikes: %v", err)
		}
	}

	return eventmodels.OptionParameterComponents{
		StockSymbol:               symbol,
		ExpirationInDays:          expirationInDays,
		Strikes:                   []int{},
		MinDistanceBetweenStrikes: minDistanceBetweenStrikes,
		MaxNoOfStrikes:            maxNoOfStrikes,
		Reason:                    reason,
	}, nil
}
