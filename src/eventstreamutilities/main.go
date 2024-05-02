package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
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

func fetchTradierOptionsByExpiration(url, bearerToken string, symbol eventmodels.StockSymbol) (*eventmodels.OptionContractDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchExpirations: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("symbol", string(symbol))
	q.Add("strikes", "true")
	q.Add("expirationType", "true")
	q.Add("contractSize", "true")

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchExpirations: failed to fetch option chain: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchExpirations: failed to fetch option chain, http code %v", res.Status)
	}

	var dto eventmodels.OptionContractDTO
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("fetchExpirations: failed to decode json: %w", err)
	}

	return &dto, nil
}

func findOptionContractsGroupedByExpiration(targetExpirationDate string, contractMap map[time.Time][]eventmodels.OptionContract) (time.Time, []eventmodels.OptionContract, error) {
	var closestContractExpDate time.Time = time.Time{}
	minDiff := int(^uint(0) >> 1) // Max int

	expDate, err := time.Parse("2006-01-02", targetExpirationDate)
	if err != nil {
		return time.Time{}, nil, err
	}

	for contractExpDate := range contractMap {
		daysUntilExpiration := int(contractExpDate.Sub(expDate).Hours() / 24)
		if daysUntilExpiration < 0 {
			daysUntilExpiration = -daysUntilExpiration
		}

		if daysUntilExpiration < minDiff {
			minDiff = daysUntilExpiration
			closestContractExpDate = contractExpDate
		}
	}

	if closestContractExpDate.IsZero() {
		return time.Time{}, nil, fmt.Errorf("no matching contract found")
	}

	return closestContractExpDate, contractMap[closestContractExpDate], nil
}

type OptionLadder struct {
	AtTheMoneyStrike float64
	CallsAboveStrike []eventmodels.OptionContract
	CallsBelowStrike []eventmodels.OptionContract
	PutsAboveStrike  []eventmodels.OptionContract
	PutsBelowStrike  []eventmodels.OptionContract
}

func splitAndSortContractsByStrike(contracts []eventmodels.OptionContract, strike float64) OptionLadder {
	var ladder OptionLadder

	for _, c := range contracts {
		switch c.OptionType {
		case eventmodels.Call:
			if c.Strike < strike {
				ladder.CallsBelowStrike = append(ladder.CallsBelowStrike, c)
			} else {
				ladder.CallsAboveStrike = append(ladder.CallsAboveStrike, c)
			}
		case eventmodels.Put:
			if c.Strike < strike {
				ladder.PutsBelowStrike = append(ladder.PutsBelowStrike, c)
			} else {
				ladder.PutsAboveStrike = append(ladder.PutsAboveStrike, c)
			}
		default:
			continue
		}
	}

	sort.Slice(ladder.CallsAboveStrike, func(i, j int) bool {
		return ladder.CallsAboveStrike[i].Strike < ladder.CallsAboveStrike[j].Strike
	})

	sort.Slice(ladder.CallsBelowStrike, func(i, j int) bool {
		return ladder.CallsBelowStrike[i].Strike > ladder.CallsBelowStrike[j].Strike
	})

	sort.Slice(ladder.PutsAboveStrike, func(i, j int) bool {
		return ladder.PutsAboveStrike[i].Strike < ladder.PutsAboveStrike[j].Strike
	})

	sort.Slice(ladder.PutsBelowStrike, func(i, j int) bool {
		return ladder.PutsBelowStrike[i].Strike > ladder.PutsBelowStrike[j].Strike
	})

	return ladder
}

func filterOptionContracts(contractMap map[time.Time][]eventmodels.OptionContract, expirationInDays []int, optionTypes []eventmodels.OptionType, maxStrikesAbove int, maxStrikesBelow int, minDistanceBetweenStrikes float64, underlyingStockPrice float64, now time.Time) ([]time.Time, []eventmodels.OptionContract) {
	allResults := make([]eventmodels.OptionContract, 0)
	var includeCalls, includePuts bool
	for _, optionType := range optionTypes {
		if optionType == eventmodels.Call {
			includeCalls = true
		} else if optionType == eventmodels.Put {
			includePuts = true
		}
	}

	var expirationDates []time.Time
	for _, days := range expirationInDays {
		targetExpirationDate := now.AddDate(0, 0, days).Format("2006-01-02")

		contractsExpirationDate, contracts, err := findOptionContractsGroupedByExpiration(targetExpirationDate, contractMap)
		if err != nil {
			continue
		}

		expirationDates = append(expirationDates, contractsExpirationDate)

		callResults := make([]eventmodels.OptionContract, 0)
		putResults := make([]eventmodels.OptionContract, 0)
		var callStrikesAbove, callStrikesBelow, putStrikesAbove, putStrikesBelow int

		ladder := splitAndSortContractsByStrike(contracts, underlyingStockPrice)

		if includeCalls {
			for _, c := range ladder.CallsBelowStrike {
				if callStrikesBelow == 0 {
					callResults = append(callResults, c)
					callStrikesBelow++
				} else if callStrikesBelow < maxStrikesBelow {
					if callResults[callStrikesBelow-1].Strike-c.Strike >= minDistanceBetweenStrikes {
						callResults = append(callResults, c)
						callStrikesBelow++
					}
				} else {
					break
				}
			}

			for _, c := range ladder.CallsAboveStrike {
				if callStrikesAbove == 0 {
					callResults = append(callResults, c)
					callStrikesAbove++
				} else if callStrikesAbove < maxStrikesAbove {
					if c.Strike-callResults[len(callResults)-1].Strike >= minDistanceBetweenStrikes {
						callResults = append(callResults, c)
						callStrikesAbove++
					}
				} else {
					break
				}
			}
		}

		if includePuts {
			for _, p := range ladder.PutsBelowStrike {
				if putStrikesBelow == 0 {
					putResults = append(putResults, p)
					putStrikesBelow++
				} else if putStrikesBelow < maxStrikesBelow {
					if putResults[putStrikesBelow-1].Strike-p.Strike >= minDistanceBetweenStrikes {
						putResults = append(putResults, p)
						putStrikesBelow++
					}
				} else {
					break
				}
			}

			for _, p := range ladder.PutsAboveStrike {
				if putStrikesAbove == 0 {
					putResults = append(putResults, p)
					putStrikesAbove++
				} else if putStrikesAbove < maxStrikesAbove {
					if p.Strike-putResults[len(putResults)-1].Strike >= minDistanceBetweenStrikes {
						putResults = append(putResults, p)
						putStrikesAbove++
					}
				} else {
					break
				}
			}
		}

		allResults = append(allResults, callResults...)
		allResults = append(allResults, putResults...)
	}

	return expirationDates, allResults
}

func fetchOptionChains(url, bearerToken string, symbol eventmodels.StockSymbol, expirations []time.Time) (map[time.Time][]*eventmodels.OptionChainTickDTO, error) {
	optionChainMapCh := make(map[time.Time][]*eventmodels.OptionChainTickDTO)

	for _, expiration := range expirations {
		expirationStr := expiration.Format("2006-01-02")

		optionChainTickDTO, err := eventservices.FetchOptionContractTicks(url, bearerToken, symbol, expirationStr)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch option chain tick: %v", err)
		}

		optionChainMapCh[expiration] = optionChainTickDTO
	}

	return optionChainMapCh, nil
}

func addAdditionInfoToOptions(requestID uuid.UUID, options []eventmodels.OptionContract, optionChainMap map[time.Time][]*eventmodels.OptionChainTickDTO) error {
	for i, option := range options {
		chain, ok := optionChainMap[option.Expiration]
		if !ok {
			return fmt.Errorf("no option chain found for expiration %s", option.Expiration.Format("2006-01-02"))
		}

		found := false

		for _, tick := range chain {
			if tick.OptionType == string(option.OptionType) && tick.Strike == option.Strike && tick.ContractSize == option.ContractSize {
				options[i].SetMetaData(&eventmodels.MetaData{RequestID: requestID})
				options[i].Symbol = eventmodels.OptionSymbol(tick.Symbol)
				options[i].Description = tick.Description
				options[i].ExpirationType = tick.ExpirationType
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("no option chain tick found for expiration %s", option.Expiration.Format("2006-01-02"))
		}
	}

	return nil
}

func fetchOptionChainWithParams(requestID uuid.UUID, optionsByExpirationURL, optionChainURL, stockURL, bearerToken string, symbol eventmodels.StockSymbol, optionTypes []eventmodels.OptionType, expirationInDays []int, minDistanceBetweenStrikes float64, maxNoOfStrikes int) ([]eventmodels.OptionContract, error) {
	optionsDTO, err := fetchTradierOptionsByExpiration(optionsByExpirationURL, bearerToken, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Tradier options: %v", err)
	}

	options, err := optionsDTO.ConvertToOptionContracts(symbol, optionTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Tradier options to contracts: %v", err)
	}

	stockTickDTO, err := eventservices.FetchStockTicks(symbol, stockURL, bearerToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stock tick: %v", err)
	}

	stockPrice := (stockTickDTO.Bid + stockTickDTO.Ask) / 2

	expirationDates, filteredOptions := filterOptionContracts(options, expirationInDays, optionTypes, maxNoOfStrikes, maxNoOfStrikes, minDistanceBetweenStrikes, stockPrice, time.Now())

	optionChainMap, err := fetchOptionChains(optionChainURL, bearerToken, symbol, expirationDates)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch option chains: %v", err)
	}

	if err := addAdditionInfoToOptions(requestID, filteredOptions, optionChainMap); err != nil {
		return nil, fmt.Errorf("failed to add symbol name to options: %v", err)
	}

	return filteredOptions, nil
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

	// Set up
	utils.InitEnvironmentVariables()
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
	case "STOP_TRACKING":
		command = 5
	case "CREATE_SIGNAL":
		command = 6
	default:
		fmt.Printf("Enter a command:\n1. List all streams\n2. Calculate all stream sizes\n3. Fetch and store Tradier options\n4. Start tracking\n5. Stop tracking\n6. Create signal\n")
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

		wg.Done()
	case 2:
		getStreamSize(ctx, esdbConn)
		wg.Done()
	case 3:
		// Setup
		esdbProducer := eventproducers.NewESDBProducer(&wg, eventStoreDBURL, []eventmodels.StreamParameter{})
		esdbProducer.Start(ctx)

		params, err := getOptionParametersComponents(nil)
		if err != nil {
			log.Fatalf("failed to get option parameters: %v", err)
		}

		existingOptionContracts, err := eventservices.FetchAll(ctx, esdbConn, &eventmodels.OptionContract{})
		if err != nil {
			log.Fatalf("failed to fetch existing contracts: %v", err)
		}

		cache := make(map[eventmodels.OptionSymbol]*eventmodels.OptionContract)
		for _, contract := range existingOptionContracts {
			cache[contract.Symbol] = contract
		}

		requestID := uuid.New()

		if _, err = FetchAndStoreTradierOptions(ctx, &wg, esdbProducer, params, cache, eventStoreDBURL, brokerCreds, requestID); err != nil {
			log.Fatalf("failed to fetch and store Tradier options: %v", err)
		}

		wg.Done()
	case 4:
		existingOptionContracts, err := eventservices.FetchAll(ctx, esdbConn, &eventmodels.OptionContract{})
		if err != nil {
			log.Fatalf("failed to fetch existing contracts: %v", err)
		}

		cache := make(map[eventmodels.OptionSymbol]*eventmodels.OptionContract)
		for _, contract := range existingOptionContracts {
			cache[contract.Symbol] = contract
		}

		// create a new tracker
		if err = StartTracking(ctx, &wg, cache, eventStoreDBURL, brokerCreds); err != nil {
			log.Fatalf("failed to start tracking: %v", err)
		}

		wg.Done()
	case 5:
		existingOptionContracts, err := eventservices.FetchAll(ctx, esdbConn, &eventmodels.OptionContract{})
		if err != nil {
			log.Fatalf("failed to fetch existing contracts: %v", err)
		}

		cache := make(map[eventmodels.OptionSymbol]*eventmodels.OptionContract)
		for _, contract := range existingOptionContracts {
			cache[contract.Symbol] = contract
		}

		if err = StopTracking(ctx, &wg, cache, eventStoreDBURL, brokerCreds); err != nil {
			log.Fatalf("failed to stop tracking: %v", err)
		}

		wg.Done()
	case 6:
		if err = CreateSignal(ctx, &wg, eventStoreDBURL); err != nil {
			log.Fatalf("failed to create signal: %v", err)
		}

		wg.Done()
	default:
		log.Fatalf("Invalid command: %d", command)
	}

	wg.Wait()
}

func StopTracking(ctx context.Context, wg *sync.WaitGroup, optionContractsCache map[eventmodels.OptionSymbol]*eventmodels.OptionContract, eventStoreDBURL string, brokerCreds BrokerCredentials) error {
	// Setup
	esdbProducer := eventproducers.NewESDBProducer(wg, eventStoreDBURL, []eventmodels.StreamParameter{})
	esdbProducer.Start(ctx)

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
	allTrackers, err := eventservices.FetchAll(ctx, esdbProducer.GetClient(), &eventmodels.Tracker{})
	if err != nil {
		return fmt.Errorf("failed to fetch all trackers: %v", err)
	}

	stopTracking := make([]*eventmodels.Tracker, 0)

	activeTrackers := eventservices.GetActiveTrackers(allTrackers)
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
		now := time.Now()

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
	esdbProducer := eventproducers.NewESDBProducer(wg, eventStoreDBURL, []eventmodels.StreamParameter{})
	esdbProducer.Start(ctx)

	allTrackers, err := eventservices.FetchAll(ctx, esdbProducer.GetClient(), &eventmodels.Tracker{})
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
	activeTrackers := eventservices.GetActiveTrackers(allTrackers)
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

	// Create signal tracker
	requestID := uuid.New()

	log.Infof("create signal requestID: %s", requestID.String())

	ts := time.Now()
	signalTracker := eventmodels.NewSignalTracker(symbol, ts, signalName, requestID)

	// Save the signal tracker
	if err := esdbProducer.Save(signalTracker); err != nil {
		return fmt.Errorf("failed to save signal tracker: %v", err)
	}

	return nil
}

func StartTracking(ctx context.Context, wg *sync.WaitGroup, optionContractsCache map[eventmodels.OptionSymbol]*eventmodels.OptionContract, eventStoreDBURL string, brokerCreds BrokerCredentials) error {
	// Setup
	esdbProducer := eventproducers.NewESDBProducer(wg, eventStoreDBURL, []eventmodels.StreamParameter{})
	esdbProducer.Start(ctx)

	allTrackers, err := eventservices.FetchAll(ctx, esdbProducer.GetClient(), &eventmodels.Tracker{})
	if err != nil {
		return fmt.Errorf("failed to fetch all trackers: %v", err)
	}

	// Check:
	activeTrackers := eventservices.GetActiveTrackers(allTrackers)

	// Check if tracker already exists
	params, err := getOptionParametersComponents(activeTrackers)
	if err != nil {
		return fmt.Errorf("failed to get option parameters: %v", err)
	}

	// Create tracker
	requestID := uuid.New()

	log.Infof("start tracking symbol: %s, requestID: %s", params.Symbol, requestID.String())

	options, err := FetchAndStoreTradierOptions(ctx, wg, esdbProducer, params, optionContractsCache, eventStoreDBURL, brokerCreds, requestID)
	if err != nil {
		return fmt.Errorf("failed to fetch and store Tradier options: %v", err)
	}

	optionContractIDs := make([]eventmodels.EventStreamID, 0)
	for _, option := range options {
		optionContractIDs = append(optionContractIDs, option.GetMetaData().GetEventStreamID())
	}

	underlyingSymbol := params.Symbol
	now := time.Now()

	tracker := eventmodels.NewStartTracker(underlyingSymbol, optionContractIDs, now, params.Reason, requestID)

	// Save the tracker
	if err := esdbProducer.Save(tracker); err != nil {
		return fmt.Errorf("failed to save tracker: %v", err)
	}

	return nil
}

func FetchAndStoreTradierOptions(ctx context.Context, wg *sync.WaitGroup, esdbProducer *eventproducers.EsdbProducer, params eventmodels.OptionParameterComponents, optionContractsCache map[eventmodels.OptionSymbol]*eventmodels.OptionContract, eventStoreDBURL string, brokerCreds BrokerCredentials, requestID uuid.UUID) ([]*eventmodels.OptionContract, error) {
	log.Infof("fetching options for symbol: %s, requestID: %s", params.Symbol, requestID.String())

	optionTypes := []eventmodels.OptionType{eventmodels.Call, eventmodels.Put}

	options, err := fetchOptionChainWithParams(requestID, brokerCreds.OptionsExpirationURL, brokerCreds.OptionChainURL, brokerCreds.StockQuotesURL, brokerCreds.BearerToken, params.Symbol, optionTypes, params.ExpirationInDays, params.MinDistanceBetweenStrikes, params.MaxNoOfStrikes)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch option chain: %v", err)
	}

	log.Infof("Saving %d option contracts ...\n", len(options))

	created := make([]*eventmodels.OptionContract, 0)
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

func getOptionParametersComponents(activeTrackers map[eventmodels.EventStreamID]*eventmodels.Tracker) (eventmodels.OptionParameterComponents, error) {
	// Get the underlying stock symbol
	var symbol eventmodels.StockSymbol
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

	var reason string
	var err error

	if activeTrackers != nil {
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
		Symbol:                    symbol,
		ExpirationInDays:          expirationInDays,
		Strikes:                   []int{},
		MinDistanceBetweenStrikes: minDistanceBetweenStrikes,
		MaxNoOfStrikes:            maxNoOfStrikes,
		Reason:                    reason,
	}, nil
}
