package eventservices

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func makePolygonAggsTickerRequestURL(symbol eventmodels.StockSymbol, timeframeValue int, timeframeUnit string, fromDate time.Time, toDate time.Time) (string, error) {
	// Parse the base URL
	parsedURL, err := url.Parse("https://api.polygon.io/v2/aggs/ticker")
	if err != nil {
		return "", fmt.Errorf("FetchPolygonStockChart: failed to parse base URL: %w", err)
	}

	// Join the additional path
	joinedPath := path.Join(parsedURL.Path, string(symbol), "range", fmt.Sprintf("%d", timeframeValue), timeframeUnit, fromDate.Format("2006-01-02"), toDate.Format("2006-01-02"))
	parsedURL.Path = joinedPath

	return parsedURL.String(), nil
}

type DailyTickerSummaryResponse struct {
	AfterHours float64 `json:"afterHours"`
	Close      float64 `json:"close"`
	From       string  `json:"from"`
	High       float64 `json:"high"`
	Low        float64 `json:"low"`
	Open       float64 `json:"open"`
	PreMarket  float64 `json:"preMarket"`
	Status     string  `json:"status"`
	Symbol     string  `json:"symbol"`
	Volume     int64   `json:"volume"`
}

func fetchPolygonDailyTickerSummary(symbol string, date eventmodels.PolygonDate, apiKey string) (*DailyTickerSummaryResponse, error) {
	url := fmt.Sprintf("https://api.polygon.io/v1/open-close/%s/%s?apiKey=%s", symbol, date.ToString(), apiKey)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchPolygonDailyTickerSummary: failed to create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchPolygonDailyTickerSummary: failed to fetch stock tick: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchPolygonDailyTickerSummary: failed to fetch stock tick, http code %v", res.Status)
	}

	var dto DailyTickerSummaryResponse
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("fetchPolygonDailyTickerSummary: failed to decode json: %w", err)
	}

	return &dto, nil
}

func fetchPolygonStockChart(url, apiKey string) (*eventmodels.PolygonCandleResponse, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchPolygonStockChart: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("sort", "asc")
	q.Add("adjusted", "false")
	q.Add("apiKey", apiKey)

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")

	// log.Tracef("fetching from %v", req.URL.String())

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchPolygonStockChart: failed to fetch stock tick: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchPolygonStockChart: failed to fetch stock tick, http code %v", res.Status)
	}

	var dto eventmodels.PolygonCandleResponse
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("fetchPolygonStockChart: failed to decode json: %w", err)
	}

	// if dto.NextURL != nil {
	// 	log.Tracef("fetchPolygonStockChart: next url: %v", *dto.NextURL)
	// }

	return &dto, nil
}

func FetchPolygonIndexChart(symbol eventmodels.StockSymbol, timeframeValue int, timeframeUnit string, fromDate time.Time, toDate time.Time, apiKey string) (*eventmodels.PolygonCandleResponse, error) {
	symbol = eventmodels.StockSymbol(fmt.Sprintf("I:%v", symbol))
	return FetchPolygonStockChart(symbol, timeframeValue, timeframeUnit, fromDate, toDate, apiKey)
}

func FetchPolygonStockChart(symbol eventmodels.StockSymbol, timeframeValue int, timeframeUnit string, fromDate time.Time, toDate time.Time, apiKey string) (*eventmodels.PolygonCandleResponse, error) {
	backOff := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 32 * time.Second, 64 * time.Second, 128 * time.Second}
	var aggregateResult eventmodels.PolygonCandleResponse

	counter := 0
	isDone := false

	var inputSymbol eventmodels.StockSymbol

	if symbol == "SPX" {
		inputSymbol = "SPY"
	} else {
		inputSymbol = symbol
	}

	for {
		url, err := makePolygonAggsTickerRequestURL(inputSymbol, timeframeValue, timeframeUnit, fromDate, toDate)
		if err != nil {
			return nil, fmt.Errorf("FetchPolygonStockChart: failed to make request URL: %w", err)
		}

		aggregateResult = eventmodels.PolygonCandleResponse{}

		if counter > 0 {
			log.Warnf("FetchPolygonStockChart: backoff %v", backOff[counter])
			time.Sleep(backOff[counter])
		}

		if counter < len(backOff)-1 {
			counter++
		}

		for {
			resp, err := fetchPolygonStockChart(url, apiKey)
			if err != nil {
				return nil, fmt.Errorf("FetchPolygonStockChart: failed to fetch stock chart: %v", err)
			}

			aggregateResult.QueryCount += resp.QueryCount
			aggregateResult.ResultsCount += resp.ResultsCount
			aggregateResult.Results = append(aggregateResult.Results, resp.Results...)

			if resp.NextURL == nil {
				isDone = true
				break
			}

			url = *resp.NextURL
			time.Sleep(50 * time.Millisecond)
		}

		if len(aggregateResult.Results) == 0 {
			return nil, fmt.Errorf("FetchPolygonStockChart: no results found from %v to %v", fromDate, toDate)
		}

		if isDone {
			break
		}
	}

	if symbol == "SPX" {
		for i := range aggregateResult.Results {
			aggregateResult.Results[i].Open *= 10
			aggregateResult.Results[i].Close *= 10
			aggregateResult.Results[i].High *= 10
			aggregateResult.Results[i].Low *= 10
			aggregateResult.Results[i].Vwap *= 10
		}
	}

	return &aggregateResult, nil
}

func FetchPolygonAggregateBars(expired bool) eventmodels.FetchDataFunc[eventmodels.PolygonAggregateBar] {
	return func(url, apiKey string) (*eventmodels.AggregateResult[eventmodels.PolygonAggregateBar], error) {
		client := http.Client{
			Timeout: 10 * time.Second,
		}

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("FetchPolygonAggregateBars: failed to create request: %w", err)
		}

		req.Header.Add("Accept", "application/json")

		q := req.URL.Query()
		q.Add("expired", fmt.Sprintf("%t", expired))
		q.Add("adjusted", "false")
		q.Add("limit", "50000")
		q.Add("sort", "asc")
		q.Add("apiKey", apiKey)

		req.URL.RawQuery = q.Encode()

		log.Debugf("FetchPolygonAggregateBars: fetching option contracts from %v", req.URL.String())

		res, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("FetchPolygonAggregateBars: failed to fetch option contracts: %w", err)
		}

		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("FetchPolygonAggregateBars: failed to fetch option contracts, http code %v", res.Status)
		}

		var dto eventmodels.PolygonGetV3ReferenceOptionsContractsResponse[eventmodels.PolygonAggregateBar]
		if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
			return nil, fmt.Errorf("FetchPolygonAggregateBars: failed to decode json: %w", err)
		}

		return &eventmodels.AggregateResult[eventmodels.PolygonAggregateBar]{
			QueryCount:   1,
			ResultsCount: len(dto.Results),
			Results:      dto.Results,
			GetNextURL:   func() *string { return dto.NextURL },
		}, nil
	}
}

// func FetchHistoricalOptionChainDataInput(symbol eventmodels.StockSymbol, timestamp time.Time, expirationGTE, expirationLTE time.Time, maxNoOfStrikes int, minDistanceBetweenStrikes float64, expirationInDays []int) (*eventmodels.FetchOptionChainDataInput, error) {
type PolygonOptionsDataFetcher struct {
	BaseURL string
	ApiKey  string
}

func NewPolygonOptionsDataFetcher(baseUrl, apiKey string) *PolygonOptionsDataFetcher {
	return &PolygonOptionsDataFetcher{
		BaseURL: baseUrl,
		ApiKey:  apiKey,
	}
}

func (fetcher *PolygonOptionsDataFetcher) FetchEVSpreads(ctx context.Context, projectDir string, signalName eventmodels.SignalName, bFindSpreads bool, startsAt, endsAt time.Time, ticker eventmodels.StockSymbol, goEnv string, options []eventmodels.OptionContractV3, stockInfo *eventmodels.StockTickItemDTO, now time.Time) (map[string]eventmodels.ExpectedProfitItemSpread, map[string]eventmodels.ExpectedProfitItemSpread, error) {
	tracer := otel.Tracer("FetchEVSpreads")
	_, span := tracer.Start(ctx, "FetchEVSpreads")
	defer span.End()

	logger := log.WithContext(ctx)

	lookaheadCandlesCount, lookaheadToOptionContractsMap := calculateLookaheadCandlesCount(now, options, 15*time.Minute)

	logger.Infof("Running %v with lookaheadCandlesCount: %v", signalName, lookaheadCandlesCount)

	switch signalName {
	case eventmodels.SuperTrend1hStochRsi15mUp:
		span.AddEvent("Executing SuperTrend1hStochRsi15mUp")
		return ExecSignalStatisicalPipelineSpreads(ctx, projectDir, lookaheadToOptionContractsMap, stockInfo, func() (eventmodels.SignalRunOutput, error) {
			return Run_Supertrend1hStochRsi15mUp(eventmodels.SupertrendRunArgs{
				StartsAt:              startsAt,
				EndsAt:                endsAt,
				Ticker:                ticker,
				LookaheadCandlesCount: lookaheadCandlesCount,
				GoEnv:                 goEnv,
			})
		})

	case eventmodels.SuperTrend1hStochRsi15mDown:
		span.AddEvent("Executing SuperTrend1hStochRsi15mDown")
		return ExecSignalStatisicalPipelineSpreads(ctx, projectDir, lookaheadToOptionContractsMap, stockInfo, func() (eventmodels.SignalRunOutput, error) {
			return Run_SuperTrend1hStochRsi15mDown(eventmodels.SupertrendRunArgs{
				StartsAt:              startsAt,
				EndsAt:                endsAt,
				Ticker:                ticker,
				LookaheadCandlesCount: lookaheadCandlesCount,
				GoEnv:                 goEnv,
			})
		})

	case eventmodels.SuperTrend4h1hStochRsi15mDown:
		span.AddEvent("Executing SuperTrend4h1hStochRsi15mDown")
		return ExecSignalStatisicalPipelineSpreads(ctx, projectDir, lookaheadToOptionContractsMap, stockInfo, func() (eventmodels.SignalRunOutput, error) {
			return Run_Supertrend4h1hStochRsi15mDown(eventmodels.SupertrendRunArgs{
				StartsAt:              startsAt,
				EndsAt:                endsAt,
				Ticker:                ticker,
				LookaheadCandlesCount: lookaheadCandlesCount,
				GoEnv:                 goEnv,
			})
		})

	case eventmodels.SuperTrend4h1hStochRsi15mUp:
		span.AddEvent("Executing SuperTrend4h1hStochRsi15mUp")
		return ExecSignalStatisicalPipelineSpreads(ctx, projectDir, lookaheadToOptionContractsMap, stockInfo, func() (eventmodels.SignalRunOutput, error) {
			return Run_Supertrend4h1hStochRsi15mUp(eventmodels.SupertrendRunArgs{
				StartsAt:              startsAt,
				EndsAt:                endsAt,
				Ticker:                ticker,
				LookaheadCandlesCount: lookaheadCandlesCount,
				GoEnv:                 goEnv,
			})
		})

	default:
		return nil, nil, fmt.Errorf("FetchEV: unknown signal name: %s", signalName)
	}
}

func filterOptionContractsV3BySymbol(contracts []eventmodels.OptionContractV3, includeSymbolPrefixes []string) []eventmodels.OptionContractV3 {
	out := make([]eventmodels.OptionContractV3, 0)

	for _, c := range contracts {
		for _, symbolPrefix := range includeSymbolPrefixes {
			optionSymbolPrefix := fmt.Sprintf("O:%s", symbolPrefix)
			if string(c.Symbol[:len(optionSymbolPrefix)]) == optionSymbolPrefix {
				out = append(out, c)
			}
		}
	}

	return out
}

func (fetcher *PolygonOptionsDataFetcher) FetchOptionChainDataInput(symbol eventmodels.StockSymbol, isHistorical bool, timestamp time.Time, expirationGTE, expirationLTE time.Time, maxNoOfStrikes int, minDistanceBetweenStrikes float64, expirationInDays []int) (*eventmodels.FetchOptionChainDataInput, error) {
	optionSpreadPerc := 0.005

	request := eventmodels.PolygonDataBulkHistOptionOHLCRequest{
		Root:                       symbol,
		ExpirationLessThanEqual:    expirationLTE,
		ExpirationGreaterThanEqual: expirationGTE,
		StartDate:                  timestamp,
		EndDate:                    timestamp,
		Interval:                   1 * time.Minute,
		Spread:                     optionSpreadPerc,
		IsExpired:                  isHistorical,
		ApiKey:                     fetcher.ApiKey,
	}

	resp, err := fetchPolygonBulkHistOptionOhlc(request)
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

	// handles an edge case where both SPX and SPXW are present
	if request.Root == "SPX" {
		contracts = filterOptionContractsV3BySymbol(contracts, []string{"SPXW"})
	}

	optionTypes := []eventmodels.OptionType{eventmodels.OptionTypeCall, eventmodels.OptionTypePut}

	optionTickByExpirationTimeMap, err := convertToTimeMap(contracts)
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to convert expiration date to time: %w", err)
	}

	stockSpreadPerc := 0.001
	closestStockTickDTO, err := FindClosestStockTickItemDTO(request, timestamp, stockSpreadPerc)
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to find closest stock tick: %w", err)
	}

	_, filteredOptions := FilterOptions(
		optionTickByExpirationTimeMap,
		closestStockTickDTO,
		expirationInDays,
		optionTypes,
		minDistanceBetweenStrikes,
		maxNoOfStrikes,
		timestamp,
	)

	marketOpen, err := eventmodels.ConvertToMarketOpen(timestamp)
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to convert to market open: %w", err)
	}

	marketClose, err := eventmodels.ConvertToMarketClose(timestamp)
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to convert to market close: %w", err)
	}

	// todo: separate bid/ask and exec req price: kindle 0122
	// nextDayMarketClose, err := eventmodels.ConvertToMarketClose(timestamp.Add(24 * time.Hour))
	// if err != nil {
	// 	return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to convert to market close: %w", err)
	// }

	polygonOptionTickDataReq := &eventmodels.PolygonOptionTickDataRequest{
		BaseURL:      fetcher.BaseURL,
		StartDate:    marketOpen,
		EndDate:      marketClose,
		Spread:       optionSpreadPerc,
		IsHistorical: isHistorical,
		ApiKey:       fetcher.ApiKey,
	}

	options, err := convertOptionsChain(
		context.Background(),
		symbol,
		filteredOptions,
		optionTickByExpirationMap,
		polygonOptionTickDataReq,
		timestamp,
	)

	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to convert options")
	}

	return &eventmodels.FetchOptionChainDataInput{
		StockTickItemDTO: closestStockTickDTO,
		OptionContracts:  options,
	}, nil
}

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

func fetchPolygonBulkHistOptionOhlc(req eventmodels.PolygonDataBulkHistOptionOHLCRequest) (*eventmodels.PolygonBulkResponse, error) {
	url := fmt.Sprintf("https://api.polygon.io/v3/reference/options/contracts")
	polygonContracts, err := utils.FetchRecursively(url, req.ApiKey, fetchPolygonReferenceOptionsContracts(req.Root, req.ExpirationGreaterThanEqual, req.ExpirationLessThanEqual, req.IsExpired))
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

func fetchPolygonReferenceOptionsContracts(symbol eventmodels.StockSymbol, expirationGreaterThanEqual, expirationLessThanEqual time.Time, isExpired bool) eventmodels.FetchDataFunc[eventmodels.PolygonOptionContract] {
	return func(url, apiKey string) (*eventmodels.AggregateResult[eventmodels.PolygonOptionContract], error) {
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
		q.Add("expired", fmt.Sprintf("%t", isExpired))
		q.Add("order", "asc")
		q.Add("limit", "1000")
		q.Add("sort", "strike_price")
		q.Add("apiKey", apiKey)

		req.URL.RawQuery = q.Encode()

		log.Debugf("fetchPolygonReferenceOptionsContracts: fetching option contracts from %v", req.URL.String())

		res, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetchPolygonReferenceOptionsContracts: failed to fetch option contracts: %w", err)
		}

		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetchPolygonReferenceOptionsContracts: failed to fetch option contracts, http code %v", res.Status)
		}

		var dto eventmodels.PolygonGetV3ReferenceOptionsContractsResponse[eventmodels.PolygonOptionContract]
		if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
			return nil, fmt.Errorf("fetchPolygonReferenceOptionsContracts: failed to decode json: %w", err)
		}

		return &eventmodels.AggregateResult[eventmodels.PolygonOptionContract]{
			QueryCount:   1,
			ResultsCount: len(dto.Results),
			Results:      dto.Results,
			GetNextURL:   func() *string { return dto.NextURL },
		}, nil
	}
}

func calculateLookaheadCandlesCount(now time.Time, options []eventmodels.OptionContractV3, candleDuration time.Duration) ([]int, map[int][]eventmodels.OptionContractV3) {
	var uniqueExpirationDates = make(map[eventmodels.ExpirationDate]eventmodels.OptionContractV3)
	lookaheadToOptionContractsMap := make(map[int][]eventmodels.OptionContractV3)

	for _, option := range options {
		uniqueExpirationDates[option.ExpirationDate] = option
	}

	lookaheadCandlesCount := []int{}
	optionExpirationToLookahead := make(map[eventmodels.ExpirationDate]int)
	for _, option := range uniqueExpirationDates {
		timeToExpiration := option.TimeUntilExpiration(now)
		if timeToExpiration.Minutes() > 0 {
			l := int(timeToExpiration.Minutes() / candleDuration.Minutes())
			lookaheadCandlesCount = append(lookaheadCandlesCount, l)
			optionExpirationToLookahead[option.ExpirationDate] = l
		}
	}

	for _, option := range options {
		if l, found := optionExpirationToLookahead[option.ExpirationDate]; found {
			lookaheadToOptionContractsMap[l] = append(lookaheadToOptionContractsMap[l], option)
		}
	}

	return lookaheadCandlesCount, lookaheadToOptionContractsMap
}
