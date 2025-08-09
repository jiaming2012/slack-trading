package main

import (
	"fmt"
	"log"
	"time"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

type OptionDatasource interface {
}

type OptionContractManager struct {
	UnderlyingSymbol string
	Clock            *models.Clock
}

type OptionContract struct {
	Symbol      string
	Expiration  time.Time
	StrikePrice float64
	OptionType  eventmodels.OptionType
}

func (m *OptionContractManager) QueryOptionContractrs(noOfStrikesBelow, noOfStrikesAbove, expirationInDays int) ([]*OptionContract, error) {

	return []*OptionContract{}, nil
}

func (m *OptionContractManager) IsExpired(contract *OptionContract) (bool, error) {
	if m.Clock == nil {
		return false, fmt.Errorf("OptionsContractManager.IsExpired: clock is not set")
	}

	return contract.Expiration.Before(m.Clock.CurrentTime), nil
}

func NewOptionsContractManager(underlyingSymbol string, clock *models.Clock) (*OptionContractManager, error) {
	if clock == nil {
		return nil, fmt.Errorf("NewOptionsContractManager: clock is not set")
	}

	return &OptionContractManager{
		UnderlyingSymbol: underlyingSymbol,
		Clock:            clock,
	}, nil
}

func main() {
	fetch_chain_yesterday()
}

func fetch_chain_yesterday() {
	projectsDir, err := utils.GetEnv("PROJECTS_DIR")
	if err != nil {
		log.Fatalf("PROJECTS_DIR not set: %v", err)
	}

	goEnv := "development"

	utils.InitEnvironmentVariables(projectsDir, goEnv)

	polygonApiKey := "7Z_3KjIHQeH3yW7RuTMGZH2kHwCK11QY"

	optionsDataFetcher := eventservices.NewPolygonOptionsClient("https://api.polygon.io", polygonApiKey)

	symbol := eventmodels.StockSymbol("XYZ")
	tz, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatalf("Failed to load timezone: %v", err)
	}

	now := time.Date(2025, 8, 7, 14, 44, 0, 0, tz) // Example date for testing
	// nextOptionsExpirationDate := utils.DeriveNextFriday(now)
	maxNoOfStrikes := 9
	minDistanceBetweenStrikes := 1.0
	expirationInDays := []int{1}

	// data, err := optionsDataFetcher.FetchOptionChainDataInput(symbol, now, now, nextOptionsExpirationDate, maxNoOfStrikes, minDistanceBetweenStrikes, expirationInDays)
	// if err != nil {
	// 	panic(fmt.Sprintf("tradier executer: %v: failed to collect data: %v", "FetchOptionChainDataInput", err))
	// }

	data, err := optionsDataFetcher.FetchOptionChainV2(symbol, now, maxNoOfStrikes, minDistanceBetweenStrikes, expirationInDays)
	if err != nil {
		panic(fmt.Sprintf("tradier executer: %v: failed to collect data: %v", "FetchOptionChainDataInput", err))
	}

	if data == nil {
		panic(fmt.Sprintf("tradier executer: %v: failed to collect data", "FetchOptionChainDataInput"))
	}

	if len(data.OptionContracts) == 0 {
		panic(fmt.Sprintf("tradier executer: %v: no option chain data", "FetchOptionChainDataInput"))
	}

	for _, contract := range data.OptionContracts {
		if contract.Symbol == "O:XYZ250808P00084000" {
			log.Printf("Found contract: %+v", contract)
		}
	}
}

func fetch_chain() {
	projectsDir, err := utils.GetEnv("PROJECTS_DIR")
	if err != nil {
		log.Fatalf("PROJECTS_DIR not set: %v", err)
	}

	goEnv := "development"

	utils.InitEnvironmentVariables(projectsDir, goEnv)

	polygonApiKey := "7Z_3KjIHQeH3yW7RuTMGZH2kHwCK11QY"

	optionsDataFetcher := eventservices.NewPolygonOptionsClient("https://api.polygon.io", polygonApiKey)

	symbol := eventmodels.StockSymbol("COIN")
	now := time.Now()
	nextOptionsExpirationDate := utils.DeriveNextFriday(now)
	maxNoOfStrikes := 4
	minDistanceBetweenStrikes := 5.0
	expirationInDays := []int{2, 5}

	data, err := optionsDataFetcher.FetchOptionChainV1(symbol, now, now, nextOptionsExpirationDate, maxNoOfStrikes, minDistanceBetweenStrikes, expirationInDays)
	if err != nil {
		panic(fmt.Sprintf("tradier executer: %v: failed to collect data: %v", "FetchOptionChainDataInput", err))
	}

	if data == nil {
		panic(fmt.Sprintf("tradier executer: %v: failed to collect data", "FetchOptionChainDataInput"))
	}

	if len(data.OptionContracts) == 0 {
		// why is this happening?
		panic(fmt.Sprintf("tradier executer: %v: no option chain data", "FetchOptionChainDataInput"))
	}

	log.Printf("Fetched option chain data for %s: %+v", symbol, data.OptionContracts)
}

func run_options_contract_manager() {
	projectsDir, err := utils.GetEnv("PROJECTS_DIR")
	if err != nil {
		log.Fatalf("PROJECTS_DIR not set: %v", err)
	}

	goEnv := "development"

	if err := utils.InitEnvironmentVariables(projectsDir, goEnv); err != nil {
		log.Panic(err)
	}

	stockQuotesURL, err := utils.GetEnv("TRADIER_STOCK_QUOTES_URL")
	if err != nil {
		log.Fatalf("$TRADIER_STOCK_QUOTES_URL not set: %v", err)
	}

	optionsExpirationURL, err := utils.GetEnv("TRADIER_OPTION_EXPIRATIONS_URL")
	if err != nil {
		log.Fatalf("$TRADIER_OPTION_EXPIRATIONS_URL not set: %v", err)
	}

	optionChainURL, err := utils.GetEnv("TRADIER_OPTION_CHAIN_URL")
	if err != nil {
		log.Fatalf("$TRADIER_OPTION_CHAIN_URL not set: %v", err)
	}

	bearerToken, err := utils.GetEnv("TRADIER_LIVE_TRADES_BEARER_TOKEN")
	if err != nil {
		log.Fatalf("$TRADIER_LIVE_TRADES_BEARER_TOKEN not set: %v", err)
	}

	symbol := eventmodels.StockSymbol("AAPL")
	optionTypes := []eventmodels.OptionType{eventmodels.OptionTypeCall, eventmodels.OptionTypePut}
	expirationInDays := []int{8}
	minDistanceBetweenStrikes := 0.5
	maxNoOfStrikes := 10

	options, stock, err := eventservices.FetchOptionChainWithParamsV2(optionsExpirationURL, optionChainURL, stockQuotesURL, bearerToken, symbol, optionTypes, expirationInDays, minDistanceBetweenStrikes, maxNoOfStrikes)
	if err != nil {
		log.Fatalf("Failed to fetch options: %v", err)
	}

	log.Printf("Fetched options for %s", symbol)
	for _, option := range options {
		log.Printf("Option: %+v", option)
	}

	log.Printf("Stock data for %s: %+v", symbol, stock)

	// Polygon API example
	polygonApiKey, err := utils.GetEnv("POLYGON_API_KEY")
	if err != nil {
		log.Fatalf("$POLYGON_API_KEY not set: %v", err)
	}

	m := eventservices.NewPolygonClient(polygonApiKey)
	timespan := eventmodels.PolygonTimespan{
		Multiplier: 15,
		Unit:       eventmodels.PolygonTimespanUnitMinute,
	}

	from := &eventmodels.PolygonDate{
		Year:  2025,
		Month: 7,
		Day:   1,
	}

	to := &eventmodels.PolygonDate{
		Year:  2025,
		Month: 7,
		Day:   31,
	}

	candles, err := m.FetchAggregateBars(eventmodels.StockSymbol("O:AAPL250808P00215000"), timespan, from, to)
	if err != nil {
		log.Fatalf("failed to fetch past aggregate bars: %v", err)
	}

	for _, candle := range candles {
		log.Printf("Candle: %+v", candle)
	}
}
