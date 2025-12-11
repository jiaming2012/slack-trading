package eventservices

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func FetchTradierMarketData(ctx context.Context, optionsByExpirationURL, stockURL, bearerToken string, symbol eventmodels.StockSymbol, optionTypes []eventmodels.OptionType) (*eventmodels.OptionContractDTO, *eventmodels.StockTickItemDTO, error) {
	tracer := otel.Tracer("FetchTradierMarketData")
	_, span := tracer.Start(ctx, "FetchTradierMarketData")
	defer span.End()

	optionsDTO, err := fetchTradierOptionsByExpiration(optionsByExpirationURL, bearerToken, symbol)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch Tradier options: %v", err)
	}

	stockTickDTO, err := FetchStockTicks(symbol, stockURL, bearerToken)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch stock tick: %v", err)
	}

	return optionsDTO, stockTickDTO, nil
}

func FilterOptions(optionContracts map[time.Time][]eventmodels.OptionContractV3, baseStrikePrice float64, expirationInDays []int, optionTypes []eventmodels.OptionType, minDistanceBetweenStrikes float64, maxNoOfStrikes int, now time.Time) ([]time.Time, []eventmodels.OptionContractV3) {
	expirationDates, filteredOptions := filterOptionContractsV3(optionContracts, expirationInDays, optionTypes, maxNoOfStrikes, maxNoOfStrikes, minDistanceBetweenStrikes, baseStrikePrice, now)
	return expirationDates, filteredOptions
}

func addAdditionalInfoToOptionsV3(options []eventmodels.OptionContractV3, optionChainMap map[eventmodels.ExpirationDate]map[eventmodels.OptionType]map[float64][]*eventmodels.OptionChainTickDTO, now time.Time) ([]eventmodels.OptionContractV3, error) {
	var resultContracts []eventmodels.OptionContractV3

	for i, option := range options {
		expirationMap, ok := optionChainMap[option.ExpirationDate]
		if !ok {
			log.Errorf("addAdditionInfoToOptionsHistoricalV3: no option chain found for expiration %s", option.Expiration.Format("2006-01-02"))
			continue
		}

		optionTypeMap, ok := expirationMap[option.OptionType]
		if !ok {
			log.Errorf("addAdditionInfoToOptionsHistoricalV3: no option chain found for option type %s", option.OptionType)
			continue
		}

		chain, ok := optionTypeMap[option.Strike]
		if !ok {
			log.Errorf("addAdditionInfoToOptionsHistoricalV3: no option chain found for strike %f", option.Strike)
			continue
		}

		// Binary search for first element with Timestamp > target
		j := sort.Search(len(chain), func(i int) bool {
			return chain[i].Timestamp.After(now)
		})

		if j > len(chain) {
			log.Errorf("addAdditionInfoToOptionsHistoricalV3: no option chain tick found for expiration %s, type %s, strike %f", option.Expiration.Format("2006-01-02"), option.OptionType, option.Strike)
			continue
		}

		if j == 0 {
			log.Errorf("addAdditionInfoToOptionsHistoricalV3: no option chain tick found for expiration %s, type %s, strike %f before the target timestamp %v", option.Expiration.Format("2006-01-02"), option.OptionType, option.Strike, now)
			continue
		}

		tick := chain[j-1] // Get the last element before the target

		exp, err := time.Parse("2006-01-02", string(option.ExpirationDate))
		if err != nil {
			log.Errorf("addAdditionInfoToOptionsHistoricalV3: failed to parse expiration date %s: %v", option.ExpirationDate, err)
			continue
		}

		exp, err = eventmodels.ConvertToMarketClose(exp)
		if err != nil {
			log.Errorf("addAdditionInfoToOptionsHistoricalV3: failed to convert expiration date to market close: %v", err)
			continue
		}

		var avgFillPrice float64

		switch option.OptionType {
		case eventmodels.OptionTypeCall:
			avgFillPrice = tick.Ask
		case eventmodels.OptionTypePut:
			avgFillPrice = tick.Bid
		default:
			log.Errorf("addAdditionInfoToOptionsHistoricalV3: invalid option type %s", option.OptionType)
			continue
		}

		options[i].Timestamp = tick.Timestamp
		options[i].Symbol = eventmodels.OptionSymbol(tick.Symbol)
		options[i].Description = tick.Description
		options[i].ExpirationType = tick.ExpirationType
		options[i].Bid = tick.Bid
		options[i].Ask = tick.Ask
		options[i].AverageFillPrice = avgFillPrice
		options[i].Expiration = exp

		if tick.Timestamp.Sub(now) > 2*time.Hour {
			log.Warnf("addAdditionInfoToOptionsHistoricalV3: %s datestamp %v that is more than 2 hours after the requested timestamp %v", options[i].Symbol, tick.Timestamp, now)
		}

		resultContracts = append(resultContracts, options[i])
	}

	return resultContracts, nil
}

func populateTickDataToOptionChainMap(contracts []eventmodels.OptionContractV3, optionChainTickMap map[eventmodels.ExpirationDate]map[eventmodels.OptionType]map[float64][]*eventmodels.OptionChainTickDTO, polygonTickDataReq *eventmodels.PolygonOptionTickDataRequest) error {
	log.Debugf("populateTickDataToOptionChainMap: start populating tick data to option chain map for %d contracts", len(contracts))

	for _, c := range contracts {
		url := fmt.Sprintf("%s/v2/aggs/ticker/%s/range/1/minute/%s/%s", polygonTickDataReq.BaseURL, c.Symbol, polygonTickDataReq.StartDate.Format("2006-01-02"), polygonTickDataReq.EndDate.Format("2006-01-02"))
		isHistorical := true
		dtos, err := utils.FetchRecursively(url, polygonTickDataReq.ApiKey, FetchPolygonAggregateBars(isHistorical))
		if err != nil {
			log.Warnf("fetchPolygonBulkHistOptionOhlc: failed to fetch data from polygon for %v: %v", c.Symbol, err)
			continue
		}

		for _, dto := range dtos.Results {
			tick := eventmodels.OptionChainTickDTO{
				Open:           dto.Open,
				Close:          dto.Close,
				High:           dto.High,
				Low:            dto.Low,
				Volume:         dto.Volume,
				OptionType:     string(c.OptionType),
				Strike:         c.Strike,
				Symbol:         string(c.Symbol),
				Timestamp:      time.UnixMilli(int64(dto.Time)),
				ContractSize:   c.ContractSize,
				ExpirationType: c.ExpirationType,
				Bid:            dto.Open,
				Ask:            dto.Open * (1 + polygonTickDataReq.Spread),
			}

			if _, ok := optionChainTickMap[c.ExpirationDate]; !ok {
				optionChainTickMap[c.ExpirationDate] = make(map[eventmodels.OptionType]map[float64][]*eventmodels.OptionChainTickDTO, 0)
			}

			if _, ok := optionChainTickMap[c.ExpirationDate][c.OptionType]; !ok {
				optionChainTickMap[c.ExpirationDate][c.OptionType] = make(map[float64][]*eventmodels.OptionChainTickDTO, 0)
			}

			if _, ok := optionChainTickMap[c.ExpirationDate][c.OptionType][c.Strike]; !ok {
				optionChainTickMap[c.ExpirationDate][c.OptionType][c.Strike] = make([]*eventmodels.OptionChainTickDTO, 0)
			}

			optionChainTickMap[c.ExpirationDate][c.OptionType][c.Strike] = append(optionChainTickMap[c.ExpirationDate][c.OptionType][c.Strike], &tick)
		}

		time.Sleep(50 * time.Millisecond) // To avoid hitting rate limits
	}

	// Sort the ticks for each expiration date, option type, and strike
	for expDate, typeMap := range optionChainTickMap {
		for optType, strikeMap := range typeMap {
			for strike, chain := range strikeMap {
				sort.Slice(chain, func(i, j int) bool {
					return chain[i].Timestamp.Before(chain[j].Timestamp)
				})
				optionChainTickMap[expDate][optType][strike] = chain
			}
		}
	}

	return nil
}

func makeOptionsChain(ctx context.Context, symbol eventmodels.StockSymbol, options []eventmodels.OptionContractV3, optionChainTicksByExpirationMap map[eventmodels.ExpirationDate]map[eventmodels.OptionType]map[float64][]*eventmodels.OptionChainTickDTO, polygonTickDataReq *eventmodels.PolygonOptionTickDataRequest, now time.Time) ([]eventmodels.OptionContractV3, error) {
	tracer := otel.Tracer("FetchOptionChainWithParamsV3")
	_, span := tracer.Start(ctx, "FetchOptionChainWithParamsV3")
	defer span.End()

	if err := populateTickDataToOptionChainMap(options, optionChainTicksByExpirationMap, polygonTickDataReq); err != nil {
		return nil, fmt.Errorf("failed to add tick data to options: %v", err)
	}

	var filteredOptions []eventmodels.OptionContractV3
	var err error
	filteredOptions, err = addAdditionalInfoToOptionsV3(options, optionChainTicksByExpirationMap, now)
	if err != nil {
		return nil, fmt.Errorf("addAdditionInfoToOptionsHistoricalV3: failed to add symbol name to options: %v", err)
	}

	return filteredOptions, nil
}

func FetchOptionChainWithParamsV2(optionsByExpirationURL, optionChainURL, stockURL, bearerToken string, symbol eventmodels.StockSymbol, optionTypes []eventmodels.OptionType, expirationInDays []int, minDistanceBetweenStrikes float64, maxNoOfStrikes int) ([]eventmodels.OptionContractV1, *eventmodels.StockTickItemDTO, error) {
	optionsDTO, err := fetchTradierOptionsByExpiration(optionsByExpirationURL, bearerToken, symbol)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch Tradier options: %v", err)
	}

	options, err := optionsDTO.ConvertToOptionContracts(symbol, optionTypes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert Tradier options to contracts: %v", err)
	}

	stockTickDTO, err := FetchStockTicks(symbol, stockURL, bearerToken)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch stock tick: %v", err)
	}

	stockPrice := (stockTickDTO.Bid + stockTickDTO.Ask) / 2

	expirationDates, filteredOptions := filterOptionContracts(options, expirationInDays, optionTypes, maxNoOfStrikes, maxNoOfStrikes, minDistanceBetweenStrikes, stockPrice, time.Now())

	optionChainMap, err := fetchOptionChains(optionChainURL, bearerToken, symbol, expirationDates)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch option chains: %v", err)
	}

	if err := addAdditionInfoToOptionsV2(filteredOptions, optionChainMap); err != nil {
		return nil, nil, fmt.Errorf("failed to add symbol name to options: %v", err)
	}

	return filteredOptions, stockTickDTO, nil
}

func FetchOptionChainWithParamsV1(requestID uuid.UUID, optionsByExpirationURL, optionChainURL, stockURL, bearerToken string, symbol eventmodels.StockSymbol, optionTypes []eventmodels.OptionType, expirationInDays []int, minDistanceBetweenStrikes float64, maxNoOfStrikes int) ([]eventmodels.OptionContractV1, error) {
	optionsDTO, err := fetchTradierOptionsByExpiration(optionsByExpirationURL, bearerToken, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Tradier options: %v", err)
	}

	options, err := optionsDTO.ConvertToOptionContracts(symbol, optionTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Tradier options to contracts: %v", err)
	}

	stockTickDTO, err := FetchStockTicks(symbol, stockURL, bearerToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stock tick: %v", err)
	}

	stockPrice := (stockTickDTO.Bid + stockTickDTO.Ask) / 2

	expirationDates, filteredOptions := filterOptionContracts(options, expirationInDays, optionTypes, maxNoOfStrikes, maxNoOfStrikes, minDistanceBetweenStrikes, stockPrice, time.Now())

	optionChainMap, err := fetchOptionChains(optionChainURL, bearerToken, symbol, expirationDates)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch option chains: %v", err)
	}

	if err := addAdditionInfoToOptionsV1(requestID, filteredOptions, optionChainMap); err != nil {
		return nil, fmt.Errorf("failed to add symbol name to options: %v", err)
	}

	return filteredOptions, nil
}
