package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/EventStore/EventStore-Client-Go/esdb"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	"slack-trading/src/eventpubsub"
	"slack-trading/src/eventservices"
)

func getStreamSize(ctx context.Context, esdbClient *esdb.Client) {
	streamNames := eventservices.ListAllStreams(ctx, esdbClient)

	for _, streamName := range streamNames {
		size, err := calculateStreamSize(ctx, esdbClient, streamName)
		if err != nil {
			log.Errorf("Error calculating size for stream %s: %v", streamName, err)
			continue
		}

		sizeInMb := float64(size) / 1024 / 1024

		fmt.Printf("Stream: %s, Size: %.2f MB\n", streamName, sizeInMb)
	}
}

func fetchTradierOptionsByExpiration(url, bearerToken string, symbol string) (*eventmodels.OptionContractDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchExpirations: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("symbol", symbol)
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

func fetchOptionChains(url, bearerToken, symbol string, expirations []time.Time) (map[time.Time][]*eventmodels.OptionChainTickDTO, error) {
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
				options[i].Symbol = tick.Symbol
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

func fetchOptionChainWithParams(requestID uuid.UUID, optionsByExpirationURL, optionChainURL, stockURL, bearerToken, symbol string, optionTypes []eventmodels.OptionType, expirationInDays []int, minDistanceBetweenStrikes float64, maxNoOfStrikes int) ([]eventmodels.OptionContract, error) {
	optionsDTO, err := fetchTradierOptionsByExpiration(optionsByExpirationURL, bearerToken, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Tradier options: %v", err)
	}

	options, err := optionsDTO.ConvertToOptionContracts(optionTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Tradier options to contracts: %v", err)
	}

	stockTickDTO, err := eventservices.FetchStockTicks("coin", stockURL, bearerToken)
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

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	// Set the connection details
	eventStoreDBURL := os.Getenv("EVENTSTOREDB_URL")
	if eventStoreDBURL == "" {
		log.Fatalf("EVENTSTOREDB_URL is required")
	} else {
		log.Infof("EventStoreDB URL: %s", eventStoreDBURL)

	}

	config, err := esdb.ParseConnectionString(eventStoreDBURL)
	if err != nil {
		log.Fatalf("Error parsing connection string: %v", err)
	}

	// Set up
	eventmodels.InitializeGlobalDispatcher()
	eventpubsub.Init()

	// Get config
	brokerBearerToken := os.Getenv("TRADIER_BEARER_TOKEN")
	stockURL := "https://sandbox.tradier.com/v1/markets/quotes"
	optionChainURL := "https://sandbox.tradier.com/v1/markets/options/chains"
	tradierOptionsExpirationURL := "https://sandbox.tradier.com/v1/markets/options/expirations"

	// Eventstore setup
	optionsContractStreamMutex := &sync.Mutex{}
	streamParams := []eventmodels.StreamParameter{
		{StreamName: eventmodels.OptionContractStream, Mutex: optionsContractStreamMutex},
	}

	esdbClient := eventproducers.NewEventStoreDBClient(&wg, streamParams)
	esdbClient.Start(ctx, eventStoreDBURL)

	// Create a new client
	esdbConn, err := esdb.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer esdbConn.Close()

	fmt.Printf("Enter a command:\n1. List all streams\n2. Calculate all stream sizes\n3. Fetch Tradier options\n")
	var command int
	fmt.Scanln(&command)
	fmt.Printf("***********************\n")

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
		handleExisingOptionContracts := func(event eventmodels.SavedEvent) {
			optionsContractStreamMutex.Unlock()
		}

		eventpubsub.Subscribe("main", eventmodels.NewSavedEvent(eventmodels.CreateOptionContractEvent), handleExisingOptionContracts)

		requestID := uuid.New()
		symbol := "coin"
		expirationInDays := []int{7, 14, 21}
		minDistanceBetweenStrikes := 10.0
		maxNoOfStrikes := 5
		optionTypes := []eventmodels.OptionType{eventmodels.Call, eventmodels.Put}

		mu := sync.Mutex{}
		savedEventsCount, err := eventservices.FindStreamLastEventNumber(esdbConn, string(eventmodels.OptionContractStream))
		if err != nil {
			log.Fatalf("Failed to find last event number: %v", err)
		}

		handleSavedOptionContractEvent := func(event eventmodels.SavedEvent) {
			mu.Lock()
			defer mu.Unlock()

			savedEventsCount--

			if savedEventsCount == 0 {
				cancel()
				return
			}

			optionsContractStreamMutex.Unlock()
		}

		options, err := fetchOptionChainWithParams(requestID, tradierOptionsExpirationURL, optionChainURL, stockURL, brokerBearerToken, symbol, optionTypes, expirationInDays, minDistanceBetweenStrikes, maxNoOfStrikes)
		if err != nil {
			log.Fatalf("Failed to fetch option chain: %v", err)
		}

		existingContracts, err := eventservices.FetchAllOptionContracts(ctx, esdbConn)
		if err != nil {
			log.Fatalf("Failed to fetch existing contracts: %v", err)
		}

		// make sure all events are read before proceeding to save new events
		<-esdbClient.AllEventsAtStartUpRead()

		// unsubscribe to old handler
		if err = eventpubsub.Unsubscribe("main", eventmodels.NewSavedEvent(eventmodels.CreateOptionContractEvent), handleExisingOptionContracts); err != nil {
			log.Fatalf("Failed to unsubscribe: %v", err)
		}

		// subscribe to new handler
		eventpubsub.Subscribe("main", eventmodels.NewSavedEvent(eventmodels.CreateOptionContractEvent), handleSavedOptionContractEvent)

		for _, option := range options {
			if _, found := existingContracts[option.Symbol]; found {
				log.Debugf("skip save: option contract %v already exists", option.ID)
				continue
			}

			mu.Lock()
			optionCopy := option
			eventpubsub.PublishAndSaveEvent("main", eventmodels.CreateOptionContractEvent, &optionCopy)
			savedEventsCount++
			mu.Unlock()

			fmt.Printf("Expiration: %s, Type: %s, Strike: %.2f\n", option.Expiration.Format("2006-01-02"), option.OptionType, option.Strike)
		}

		fmt.Println("Done")
	default:
		log.Fatalf("Invalid command: %d", command)
	}

	wg.Wait()
}

func calculateStreamSize(ctx context.Context, esdbClient *esdb.Client, streamName string) (int64, error) {
	var size int64
	readOptions := esdb.ReadStreamOptions{
		Direction: esdb.Forwards,
		From:      esdb.Start{},
	}

	count := 0
	fetchSize := 4096
	lastEventNo, err := eventservices.FindStreamLastEventNumber(esdbClient, streamName)
	if err != nil {
		return 0, fmt.Errorf("failed to find last event number: %v", err)
	}

	if lastEventNo == 0 {
		return 0, nil
	}

	terminalEventNumber := int(lastEventNo)

	for count < terminalEventNumber {
		stream, err := esdbClient.ReadStream(ctx, streamName, readOptions, uint64(fetchSize))
		if err != nil {
			return 0, err
		}
		defer stream.Close()

		for {
			event, err := stream.Recv()
			if err != nil {
				break
			}
			size += int64(len(event.Event.Data))
			size += int64(len(event.Event.UserMetadata))
			size += int64(len(event.Event.SystemMetadata))
		}

		count += fetchSize
	}

	return size, nil
}
