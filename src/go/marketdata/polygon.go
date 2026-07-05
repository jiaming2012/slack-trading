package marketdata

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/utils"
)

// polygonHTTPClient is a shared HTTP client for Polygon API calls.
// It forces HTTP/1.1 to avoid HTTP/2 GOAWAY errors from Polygon's server
// which closes connections after ~199 streams.
var polygonHTTPClient = &http.Client{
	Timeout: 45 * time.Second,
	Transport: &http.Transport{
		TLSNextProto:        make(map[string]func(authority string, c *tls.Conn) http.RoundTripper), // disable HTTP/2
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
}

func makePolygonAggsTickerRequestURL(symbol models.StockSymbol, timeframeValue int, timeframeUnit string, fromDate time.Time, toDate time.Time) (string, error) {
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

func fetchPolygonDailyTickerSummary(symbol string, date models.PolygonDate, apiKey string) (*DailyTickerSummaryResponse, error) {
	url := fmt.Sprintf("https://api.polygon.io/v1/open-close/%s/%s?apiKey=%s", symbol, date.ToString(), apiKey)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchPolygonDailyTickerSummary: failed to create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")

	res, err := polygonHTTPClient.Do(req)
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

func fetchPolygonStockChart(url, apiKey string) (*models.PolygonCandleResponse, error) {
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

	res, err := polygonHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchPolygonStockChart: failed to fetch stock tick: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchPolygonStockChart: failed to fetch stock tick, http code %v", res.Status)
	}

	var dto models.PolygonCandleResponse
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("fetchPolygonStockChart: failed to decode json: %w", err)
	}

	// if dto.NextURL != nil {
	// 	log.Tracef("fetchPolygonStockChart: next url: %v", *dto.NextURL)
	// }

	return &dto, nil
}

func FetchPolygonIndexChart(symbol models.StockSymbol, timeframeValue int, timeframeUnit string, fromDate time.Time, toDate time.Time, apiKey string) (*models.PolygonCandleResponse, error) {
	symbol = models.StockSymbol(fmt.Sprintf("I:%v", symbol))
	return FetchPolygonStockChart(symbol, timeframeValue, timeframeUnit, fromDate, toDate, apiKey)
}

func FetchPolygonOptionAggregateBars(symbol string, from time.Time, to *time.Time, apiKey string) (*models.AggregateResult[models.PolygonAggregateBar], error) {
	now := time.Now()
	if from.After(now) { // fixes api error when from is in the future
		from = now.AddDate(0, 0, -1)
	}

	var toTimestamp time.Time
	if to == nil {
		toTimestamp = time.Now()
	} else {
		toTimestamp = *to
	}

	url := fmt.Sprintf("https://api.polygon.io/v2/aggs/ticker/%s/range/1/minute/%s/%s?apiKey=%s", symbol, from.Format("2006-01-02"), toTimestamp.Format("2006-01-02"), apiKey)
	return FetchPolygonAggregateBars(false)(url, apiKey)
}

func FetchPolygonStockChart(symbol models.StockSymbol, timeframeValue int, timeframeUnit string, fromDate time.Time, toDate time.Time, apiKey string) (*models.PolygonCandleResponse, error) {
	backOff := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 32 * time.Second, 64 * time.Second, 128 * time.Second}
	var aggregateResult models.PolygonCandleResponse

	counter := 0
	isDone := false

	var inputSymbol models.StockSymbol

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

		aggregateResult = models.PolygonCandleResponse{}

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
				// Retry transient errors (GOAWAY, connection reset)
				retryBackoff := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}
				retried := false
				for attempt := 0; attempt < len(retryBackoff); attempt++ {
					log.Warnf("FetchPolygonStockChart: transient error (attempt %d/%d): %v — retrying in %v", attempt+1, len(retryBackoff), err, retryBackoff[attempt])
					time.Sleep(retryBackoff[attempt])
					resp, err = fetchPolygonStockChart(url, apiKey)
					if err == nil {
						retried = true
						break
					}
				}
				if !retried {
					return nil, fmt.Errorf("FetchPolygonStockChart: failed to fetch stock chart after retries: %v", err)
				}
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

func FetchPolygonAggregateBars(expired bool) models.FetchDataFunc[models.PolygonAggregateBar] {
	return func(url, apiKey string) (*models.AggregateResult[models.PolygonAggregateBar], error) {
		client := http.Client{
			Timeout: 45 * time.Second,
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

		var dto models.PolygonGetV3ReferenceOptionsContractsResponse[models.PolygonAggregateBar]
		if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
			return nil, fmt.Errorf("FetchPolygonAggregateBars: failed to decode json: %w", err)
		}

		if dto.Status == "DELAYED" {
			log.Warnf("FetchPolygonAggregateBars: (%d results) response status is DELAYED, this might be due to the API rate limit or other issues. URL: %s", len(dto.Results), req.URL.String())
		}

		return &models.AggregateResult[models.PolygonAggregateBar]{
			QueryCount:   1,
			ResultsCount: len(dto.Results),
			Results:      dto.Results,
			GetNextURL:   func() *string { return dto.NextURL },
		}, nil
	}
}

// func FetchHistoricalOptionChainDataInput(symbol models.StockSymbol, timestamp time.Time, expirationGTE, expirationLTE time.Time, maxNoOfStrikes int, minDistanceBetweenStrikes float64, expirationInDays []int) (*models.FetchOptionChainDataInput, error) {
type PolygonOptionsClient struct {
	BaseURL string
	ApiKey  string
	Cache   *PolygonCache
}

func NewPolygonOptionsClient(baseUrl, apiKey string, cacheDir ...string) *PolygonOptionsClient {
	var dir string
	if len(cacheDir) > 0 {
		dir = cacheDir[0]
	}
	return &PolygonOptionsClient{
		BaseURL: baseUrl,
		ApiKey:  apiKey,
		Cache:   NewPolygonCache(dir),
	}
}

func (fetcher *PolygonOptionsClient) ExerciseOption(ctx context.Context, req *models.ExerciseOptionRequest) error {
	return fmt.Errorf("ExerciseOption: not implemented for live polygon options broker")
}

func (fetcher *PolygonOptionsClient) GetCandles(playgroundID uuid.UUID, symbol models.OptionSymbol, period time.Duration, from time.Time, to *time.Time) ([]*models.AggregateBarWithIndicators, error) {
	candles, err := FetchOptionCandles(fetcher, playgroundID, symbol, period, from, to)
	if err != nil {
		return nil, fmt.Errorf("PolygonOptionsClient.GetCandles: failed to fetch option candles: %w", err)
	}

	return candles, nil
}

func (fetcher *PolygonOptionsClient) FetchPolygonOptionAggregateBars(playgroundID uuid.UUID, symbol models.OptionSymbol, period time.Duration, from time.Time, to *time.Time) (*models.AggregateResult[models.PolygonAggregateBar], error) {
	return FetchPolygonOptionAggregateBars(string(symbol), from, to, fetcher.ApiKey)
}

func (fetcher *PolygonOptionsClient) FetchEVSpreads(ctx context.Context, projectDir string, signalName models.SignalName, bFindSpreads bool, startsAt, endsAt time.Time, ticker models.StockSymbol, goEnv string, options []models.OptionContractV3, stockInfo *models.StockTickItemDTO, now time.Time) (map[string]models.ExpectedProfitItemSpread, map[string]models.ExpectedProfitItemSpread, error) {
	tracer := otel.Tracer("FetchEVSpreads")
	_, span := tracer.Start(ctx, "FetchEVSpreads")
	defer span.End()

	logger := log.WithContext(ctx)

	lookaheadCandlesCount, lookaheadToOptionContractsMap := calculateLookaheadCandlesCount(now, options, 15*time.Minute)

	logger.Infof("Running %v with lookaheadCandlesCount: %v", signalName, lookaheadCandlesCount)

	switch signalName {
	case models.SuperTrend1hStochRsi15mUp:
		span.AddEvent("Executing SuperTrend1hStochRsi15mUp")
		return ExecSignalStatisicalPipelineSpreads(ctx, projectDir, lookaheadToOptionContractsMap, stockInfo, func() (models.SignalRunOutput, error) {
			return Run_Supertrend1hStochRsi15mUp(models.SupertrendRunArgs{
				StartsAt:              startsAt,
				EndsAt:                endsAt,
				Ticker:                ticker,
				LookaheadCandlesCount: lookaheadCandlesCount,
				GoEnv:                 goEnv,
			})
		})

	case models.SuperTrend1hStochRsi15mDown:
		span.AddEvent("Executing SuperTrend1hStochRsi15mDown")
		return ExecSignalStatisicalPipelineSpreads(ctx, projectDir, lookaheadToOptionContractsMap, stockInfo, func() (models.SignalRunOutput, error) {
			return Run_SuperTrend1hStochRsi15mDown(models.SupertrendRunArgs{
				StartsAt:              startsAt,
				EndsAt:                endsAt,
				Ticker:                ticker,
				LookaheadCandlesCount: lookaheadCandlesCount,
				GoEnv:                 goEnv,
			})
		})

	case models.SuperTrend4h1hStochRsi15mDown:
		span.AddEvent("Executing SuperTrend4h1hStochRsi15mDown")
		return ExecSignalStatisicalPipelineSpreads(ctx, projectDir, lookaheadToOptionContractsMap, stockInfo, func() (models.SignalRunOutput, error) {
			return Run_Supertrend4h1hStochRsi15mDown(models.SupertrendRunArgs{
				StartsAt:              startsAt,
				EndsAt:                endsAt,
				Ticker:                ticker,
				LookaheadCandlesCount: lookaheadCandlesCount,
				GoEnv:                 goEnv,
			})
		})

	case models.SuperTrend4h1hStochRsi15mUp:
		span.AddEvent("Executing SuperTrend4h1hStochRsi15mUp")
		return ExecSignalStatisicalPipelineSpreads(ctx, projectDir, lookaheadToOptionContractsMap, stockInfo, func() (models.SignalRunOutput, error) {
			return Run_Supertrend4h1hStochRsi15mUp(models.SupertrendRunArgs{
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

func filterOptionContractsV3BySymbol(contracts []models.OptionContractV3, includeSymbolPrefixes []string) []models.OptionContractV3 {
	out := make([]models.OptionContractV3, 0)

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

func (fetcher *PolygonOptionsClient) maxExpirationInDays(expirationInDays []int) int {
	if len(expirationInDays) == 0 {
		return 0
	}

	max := expirationInDays[0]
	for _, days := range expirationInDays {
		if days > max {
			max = days
		}
	}

	return max
}

func (fetcher *PolygonOptionsClient) FetchOptionChainV2(symbol models.StockSymbol, timestamp time.Time, maxNoOfStrikes int, minDistanceBetweenStrikes float64, expirationInDays []int, maxTickAge time.Duration, baseStrikePrice *float64, calendarRepo models.CalendarRepository) (*models.FetchOptionChainDataInput, error) {
	expirationGTE := timestamp

	maxDays := fetcher.maxExpirationInDays(expirationInDays)
	expirationLTE := utils.DeriveNextFriday(timestamp.AddDate(0, 0, maxDays))

	return fetcher.FetchOptionChainV1(symbol, timestamp, expirationGTE, expirationLTE, maxNoOfStrikes, minDistanceBetweenStrikes, expirationInDays, maxTickAge, baseStrikePrice, calendarRepo)
}

func (fetcher *PolygonOptionsClient) FetchOptionChainV1(symbol models.StockSymbol, timestamp time.Time, expirationGTE, expirationLTE time.Time, maxNoOfStrikes int, minDistanceBetweenStrikes float64, expirationInDays []int, maxTickAge time.Duration, baseStrikePrice *float64, calendarRepo models.CalendarRepository) (*models.FetchOptionChainDataInput, error) {
	if maxTickAge <= 0 {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: maxTickAge must be greater than 0")
	}

	optionSpreadPerc := 0.005

	request := models.PolygonDataBulkHistOptionOHLCRequest{
		Root:                       symbol,
		ExpirationLessThanEqual:    expirationLTE,
		ExpirationGreaterThanEqual: expirationGTE,
		StartDate:                  timestamp,
		EndDate:                    timestamp,
		Interval:                   1 * time.Minute,
		Spread:                     optionSpreadPerc,
		ApiKey:                     fetcher.ApiKey,
	}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to load location: %w", err)
	}

	// --- Cached fetchPolygonBulkHistOptionOhlc (active contracts) ---
	request.IsExpired = false
	resp := fetcher.Cache.GetContracts(symbol, expirationGTE, expirationLTE, false)
	if resp == nil {
		resp, err = fetchPolygonBulkHistOptionOhlc(request)
		if err != nil {
			return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to fetch option ohlc: %w", err)
		}
		fetcher.Cache.SetContracts(symbol, expirationGTE, expirationLTE, false, resp)
	}

	// --- Cached fetchPolygonBulkHistOptionOhlc (expired contracts) ---
	request.IsExpired = true
	respExpired := fetcher.Cache.GetContracts(symbol, expirationGTE, expirationLTE, true)
	if respExpired == nil {
		respExpired, err = fetchPolygonBulkHistOptionOhlc(request)
		if err != nil {
			return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to fetch expired option: %w", err)
		}
		fetcher.Cache.SetContracts(symbol, expirationGTE, expirationLTE, true, respExpired)
	}

	// Deep-copy then merge so we don't mutate the cached values
	merged := resp.DeepCopy()
	merged.Merge(respExpired)

	contracts, optionTickByExpirationMap, err := merged.GetOptionContractsV3(loc, optionSpreadPerc)
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to get option contracts: %w", err)
	}

	// handles an edge case where both SPX and SPXW are present
	if request.Root == "SPX" {
		contracts = filterOptionContractsV3BySymbol(contracts, []string{"SPXW"})
	}

	optionTypes := []models.OptionType{models.OptionTypeCall, models.OptionTypePut}

	optionTickByExpirationTimeMap, err := convertToTimeMap(contracts)
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to convert expiration date to time: %w", err)
	}

	// --- Cached FindClosestStockTickItemDTO ---
	stockSpreadPerc := 0.001
	closestStockTickDTO := fetcher.Cache.GetStockTick(symbol, timestamp)
	if closestStockTickDTO == nil {
		closestStockTickDTO, err = FindClosestStockTickItemDTO(request, timestamp, stockSpreadPerc)
		if err != nil {
			return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to find closest stock tick: %w", err)
		}
		fetcher.Cache.SetStockTick(symbol, timestamp, closestStockTickDTO)
	}

	if baseStrikePrice == nil {
		avgCurrentPrc := (closestStockTickDTO.Bid + closestStockTickDTO.Ask) / 2
		baseStrikePrice = &avgCurrentPrc
	}

	_, filteredOptions := FilterOptions(
		optionTickByExpirationTimeMap,
		*baseStrikePrice,
		expirationInDays,
		optionTypes,
		minDistanceBetweenStrikes,
		maxNoOfStrikes,
		timestamp,
	)

	marketOpen, err := models.ConvertToMarketOpen(timestamp)
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to convert to market open: %w", err)
	}

	marketClose, err := models.ConvertToMarketClose(timestamp)
	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to convert to market close: %w", err)
	}

	polygonOptionTickDataReq := &models.PolygonOptionTickDataRequest{
		BaseURL:   fetcher.BaseURL,
		StartDate: marketOpen.AddDate(0, 0, -3),
		EndDate:   marketClose,
		Spread:    optionSpreadPerc,
		ApiKey:    fetcher.ApiKey,
	}

	options, err := makeOptionsChain(
		context.Background(),
		symbol,
		filteredOptions,
		optionTickByExpirationMap,
		polygonOptionTickDataReq,
		timestamp,
		fetcher.Cache,
	)

	if err != nil {
		return nil, fmt.Errorf("FetchHistoricalOptionChainDataInput: failed to convert options")
	}

	// use the option's timestamp filter out data that is too old
	options = filterOptionsBeforeTime(options, timestamp, maxTickAge, calendarRepo)

	return &models.FetchOptionChainDataInput{
		StockTickItemDTO: closestStockTickDTO,
		OptionContracts:  options,
	}, nil
}

func filterOptionsBeforeTime(contracts []models.OptionContractV3, targetTime time.Time, threshold time.Duration, calendarRepo models.CalendarRepository) []models.OptionContractV3 {
	filtered := make([]models.OptionContractV3, 0)

	for _, c := range contracts {
		// skip if the option's timestamp + threshold is before the target time, unless timestamp + threshold
		// occurs when the market is closed
		optionTimestamp := c.Timestamp.Add(threshold)
		isOpen := calendarRepo.IsMarketOpen(optionTimestamp)

		if !isOpen {
			calendar, err := calendarRepo.GetNextMarketOpen(optionTimestamp)
			if err == nil {
				optionTimestamp = calendar.MarketOpen
			} else {
				log.Warnf("filterOptionsBeforeTime: failed to get next market open for option %v at time %v: %v", c.Symbol, optionTimestamp, err)
			}
		}

		if optionTimestamp.Before(targetTime) {
			continue
		}

		filtered = append(filtered, c)
	}

	return filtered
}

func convertToTimeMap(contracts []models.OptionContractV3) (map[time.Time][]models.OptionContractV3, error) {
	result := make(map[time.Time][]models.OptionContractV3)
	for _, c := range contracts {
		ts, err := time.Parse("2006-01-02", string(c.ExpirationDate))
		if err != nil {
			log.Fatalf("failed to parse expiration date: %v", err)
		}

		if _, ok := result[ts]; !ok {
			result[ts] = make([]models.OptionContractV3, 0)
		}

		result[ts] = append(result[ts], c)
	}

	return result, nil
}

func fetchPolygonBulkHistOptionOhlc(req models.PolygonDataBulkHistOptionOHLCRequest) (*models.PolygonBulkResponse, error) {
	url := "https://api.polygon.io/v3/reference/options/contracts"
	polygonContracts, err := utils.FetchRecursively(url, req.ApiKey, fetchPolygonReferenceOptionsContracts(req.Root, req.ExpirationGreaterThanEqual, req.ExpirationLessThanEqual, req.IsExpired))
	if err != nil {
		return nil, fmt.Errorf("fetchPolygonBulkHistOptionOhlc: failed to fetch option contracts: %w", err)
	}

	var contracts []models.OptionContractV3
	for _, c := range polygonContracts.Results {
		expiration, err := time.Parse("2006-01-02", c.ExpirationDate)
		if err != nil {
			return nil, fmt.Errorf("fetchPolygonBulkHistOptionOhlc: failed to parse expiration date %s: %w", c.ExpirationDate, err)
		}

		contract := models.OptionContractV3{
			ExpirationDate:   models.ExpirationDate(c.ExpirationDate),
			Expiration:       expiration,
			OptionType:       c.ContractType,
			Strike:           c.StrikePrice,
			ContractSize:     c.SharesPerContract,
			Symbol:           c.Ticker,
			UnderlyingSymbol: c.UnderlyingTicker,
		}

		contracts = append(contracts, contract)
	}

	ticksMap := make(map[models.ExpirationDate]map[models.OptionType]map[float64][]*models.OptionChainTickDTO)

	return &models.PolygonBulkResponse{
		Contracts: contracts,
		TicksMap:  ticksMap,
	}, nil
}

func fetchPolygonReferenceOptionsContracts(symbol models.StockSymbol, expirationGreaterThanEqual, expirationLessThanEqual time.Time, isExpired bool) models.FetchDataFunc[models.PolygonOptionContract] {
	return func(url, apiKey string) (*models.AggregateResult[models.PolygonOptionContract], error) {
		client := http.Client{
			Timeout: 45 * time.Second,
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

		var dto models.PolygonGetV3ReferenceOptionsContractsResponse[models.PolygonOptionContract]
		if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
			return nil, fmt.Errorf("fetchPolygonReferenceOptionsContracts: failed to decode json: %w", err)
		}

		return &models.AggregateResult[models.PolygonOptionContract]{
			QueryCount:   1,
			ResultsCount: len(dto.Results),
			Results:      dto.Results,
			GetNextURL:   func() *string { return dto.NextURL },
		}, nil
	}
}

func calculateLookaheadCandlesCount(now time.Time, options []models.OptionContractV3, candleDuration time.Duration) ([]int, map[int][]models.OptionContractV3) {
	var uniqueExpirationDates = make(map[models.ExpirationDate]models.OptionContractV3)
	lookaheadToOptionContractsMap := make(map[int][]models.OptionContractV3)

	for _, option := range options {
		uniqueExpirationDates[option.ExpirationDate] = option
	}

	lookaheadCandlesCount := []int{}
	optionExpirationToLookahead := make(map[models.ExpirationDate]int)
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
