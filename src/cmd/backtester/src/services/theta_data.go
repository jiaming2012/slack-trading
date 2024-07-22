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

func FetchHistoricalOptionChainDataInput(event *eventconsumers.SignalTriggeredEvent, exp time.Time, maxNoOfStrikes int, minDistanceBetweenStrikes float64, expirationInDays []int) (*optionsapi.FetchOptionChainDataInput, error) {
	request := eventmodels.ThetaDataBulkHistOptionOHLCRequest{
		Root:       event.Symbol,
		Expiration: exp,
		StartDate:  event.Timestamp,
		EndDate:    event.Timestamp,
		Interval:   1 * time.Minute,
	}

	baseURL := "http://localhost:25510"
	resp, err := fetchOptionThetaBulkHistOptionOhlc(baseURL, request)
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to fetch option ohlc: %w", err)
	}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to load location: %w", err)
	}

	optionSpreadPerc := 0.005
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

	options, err := eventservices.ConvertOptionsChain(
		context.Background(),
		event.Symbol,
		filteredOptions,
		optionTickByExpirationMap,
	)

	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to convert options")
	}

	return &optionsapi.FetchOptionChainDataInput{
		StockTickItemDTO: closestStockTickDTO,
		OptionContracts:  options,
	}, nil
}
