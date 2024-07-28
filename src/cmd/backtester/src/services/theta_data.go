package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventconsumers"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventproducers/optionsapi"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func convertToTimeMap(contracts []eventmodels.OptionContractV3) (map[time.Time][]eventmodels.OptionContractV3, error) {
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

func fetchOptionThetaBulkHistOptionOhlc(baseURL string, r eventmodels.ThetaDataBulkHistOptionOHLCRequest) (*eventmodels.ThetaDataBulkResponse, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	url := fmt.Sprintf("%s/v2/bulk_hist/option/ohlc", baseURL)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchOptionThetaBulkHistOptionOhlc: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("root", string(r.Root))
	q.Add("exp", r.Expiration.Format("20060102"))
	q.Add("start_date", r.StartDate.Format("20060102"))
	q.Add("end_date", r.EndDate.Format("20060102"))
	q.Add("ivl", fmt.Sprintf("%d", (int(r.Interval/time.Minute)*60000)))

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")

	log.Printf("fetchOptionThetaBulkHistOptionOhlc: fetching option ohlc from %v", req.URL.String())

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchOptionThetaBulkHistOptionOhlc: failed to fetch option ohlc: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchOptionThetaBulkHistOptionOhlc: failed to fetch option ohlc, http code %v", res.Status)
	}

	var dto eventmodels.ThetaDataBulkResponse
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("fetchOptionThetaBulkHistOptionOhlc: failed to decode json: %w", err)
	}

	return &dto, nil
}

type PolygonOptionContract struct {
	ContractType      eventmodels.OptionType   `json:"contract_type"`
	ExerciseStyle     string                   `json:"exercise_style"`
	ExpirationDate    string                   `json:"expiration_date"`
	SharesPerContract int                      `json:"shares_per_contract"`
	StrikePrice       float64                  `json:"strike_price"`
	Ticker            eventmodels.OptionSymbol `json:"ticker"`
	UnderlyingTicker  eventmodels.StockSymbol  `json:"underlying_ticker"`
}

func fetchPolygonReferenceOptionsContracts(symbol eventmodels.StockSymbol, expirationGreaterThanEqual, expirationLessThanEqual time.Time) eventmodels.FetchDataFunc[PolygonOptionContract] {
	return func(url, apiKey string) (*eventmodels.AggregateResult[PolygonOptionContract], error) {
		client := http.Client{
			Timeout: 10 * time.Second,
		}

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("fetchPolygonReferenceOptionsContracts: failed to create request: %w", err)
		}

		req.Header.Add("Accept", "application/json")

		q := req.URL.Query()
		q.Add("underlying_ticker", string(symbol))
		q.Add("expiration_date.gte", expirationGreaterThanEqual.Format("2006-01-02"))
		q.Add("expiration_date.lte", expirationLessThanEqual.Format("2006-01-02"))
		q.Add("expired", "true")
		q.Add("order", "asc")
		q.Add("limit", "1000")
		q.Add("sort", "strike_price")
		q.Add("apiKey", apiKey)

		req.URL.RawQuery = q.Encode()

		log.Infof("fetchPolygonReferenceOptionsContracts: fetching option contracts from %v", req.URL.String())

		res, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetchPolygonReferenceOptionsContracts: failed to fetch option contracts: %w", err)
		}

		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetchPolygonReferenceOptionsContracts: failed to fetch option contracts, http code %v", res.Status)
		}

		var dto eventmodels.PolygonGetV3ReferenceOptionsContractsResponse[PolygonOptionContract]
		if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
			return nil, fmt.Errorf("fetchPolygonReferenceOptionsContracts: failed to decode json: %w", err)
		}

		return &eventmodels.AggregateResult[PolygonOptionContract]{
			QueryCount:   1,
			ResultsCount: len(dto.Results),
			Results:      dto.Results,
			GetNextURL:   func() *string { return dto.NextURL },
		}, nil
	}
}

func fetchPolygonBulkHistOptionOhlc(baseURL string, req eventmodels.PolygonDataBulkHistOptionOHLCRequest) (*eventmodels.PolygonBulkResponse, error) {
	url := fmt.Sprintf("%s/v3/reference/options/contracts", baseURL)
	polygonContracts, err := utils.FetchRecursively(url, fetchPolygonReferenceOptionsContracts(req.Root, req.ExpirationGreaterThanEqual, req.ExpirationLessThanEqual))
	if err != nil {
		return nil, fmt.Errorf("fetchPolygonBulkHistOptionOhlc: failed to fetch option contracts: %w", err)
	}

	var contracts []eventmodels.OptionContractV3
	for _, c := range polygonContracts.Results {
		contract := eventmodels.OptionContractV3{
			ExpirationDate:   eventmodels.ExpirationDate(c.ExpirationDate),
			OptionType:       c.ContractType,
			Strike:           c.StrikePrice,
			ContractSize:     c.SharesPerContract,
			Symbol:           c.Ticker,
			UnderlyingSymbol: c.UnderlyingTicker,
		}

		contracts = append(contracts, contract)
	}

	ticksMap := make(map[eventmodels.ExpirationDate][]*eventmodels.OptionChainTickDTO)

	return &eventmodels.PolygonBulkResponse{
		Contracts: contracts,
		TicksMap:  ticksMap,
	}, nil
}

func FetchHistoricalOptionChainDataInput(event *eventconsumers.SignalTriggeredEvent, expirationGTE, expirationLTE time.Time, maxNoOfStrikes int, minDistanceBetweenStrikes float64, expirationInDays []int) (*optionsapi.FetchOptionChainDataInput, error) {
	optionSpreadPerc := 0.005

	request := eventmodels.PolygonDataBulkHistOptionOHLCRequest{
		Root:                       event.Symbol,
		ExpirationLessThanEqual:    expirationLTE,
		ExpirationGreaterThanEqual: expirationGTE,
		StartDate:                  event.Timestamp,
		EndDate:                    event.Timestamp,
		Interval:                   1 * time.Minute,
		Spread:                     optionSpreadPerc,
	}

	baseURL := "https://api.polygon.io"
	resp, err := fetchPolygonBulkHistOptionOhlc(baseURL, request)
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to fetch option ohlc: %w", err)
	}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to load location: %w", err)
	}

	contracts, optionTickByExpirationMap, err := resp.GetOptionContractsV3(loc, optionSpreadPerc)
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to get option contracts: %w", err)
	}

	optionTypes := []eventmodels.OptionType{eventmodels.OptionTypeCall, eventmodels.OptionTypePut}

	optionTickByExpirationTimeMap, err := convertToTimeMap(contracts)
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to convert expiration date to time: %w", err)
	}

	stockSpreadPerc := 0.001
	closestStockTickDTO, err := FindClosestStockTickItemDTO(request, event.Timestamp, stockSpreadPerc)
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to find closest stock tick: %w", err)
	}

	_, filteredOptions := eventservices.FilterOptions(
		optionTickByExpirationTimeMap,
		closestStockTickDTO,
		expirationInDays,
		optionTypes,
		minDistanceBetweenStrikes,
		maxNoOfStrikes,
		event.Timestamp,
	)

	polygonOptionTickDataReq := &eventmodels.PolygonOptionTickDataRequest{
		BaseURL:   baseURL,
		StartDate: event.Timestamp,
		EndDate:   event.Timestamp,
		Spread:    optionSpreadPerc,
	}

	options, err := eventservices.ConvertOptionsChain(
		context.Background(),
		event.Symbol,
		filteredOptions,
		optionTickByExpirationMap,
		polygonOptionTickDataReq,
		event.Timestamp,
	)

	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to convert options")
	}

	return &optionsapi.FetchOptionChainDataInput{
		StockTickItemDTO: closestStockTickDTO,
		OptionContracts:  options,
	}, nil
}
