package eventservices

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func FetchStandardDeviation(url string, bearerToken string, symbol eventmodels.StockSymbol, now time.Time) (float64, error) {
	endDate := now.Add(-24 * time.Hour)
	startDate := endDate.Add(-1 * (time.Hour * 24 * 365))

	candles, err := fetchTradierHistoricalPrices(url, bearerToken, symbol, startDate, endDate)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch historical prices: %v", err)
	}

	fmt.Printf("fetched candles: %v\n", len(candles))

	return 0, nil
}

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

func FilterOptions(optionContracts map[time.Time][]eventmodels.OptionContractV3, stockTickDTO *eventmodels.StockTickItemDTO, expirationInDays []int, optionTypes []eventmodels.OptionType, minDistanceBetweenStrikes float64, maxNoOfStrikes int, now time.Time) ([]time.Time, []eventmodels.OptionContractV3) {
	stockPrice := (stockTickDTO.Bid + stockTickDTO.Ask) / 2

	expirationDates, filteredOptions := filterOptionContractsV3(optionContracts, expirationInDays, optionTypes, maxNoOfStrikes, maxNoOfStrikes, minDistanceBetweenStrikes, stockPrice, now)

	return expirationDates, filteredOptions
}

func addAdditionInfoToOptionsV3(options []eventmodels.OptionContractV3, optionChainMap map[eventmodels.ExpirationDate][]*eventmodels.OptionChainTickDTO) error {
	for i, option := range options {
		chain, ok := optionChainMap[option.ExpirationDate]
		if !ok {
			return fmt.Errorf("addAdditionInfoToOptionsV3: no option chain found for expiration %s", option.Expiration.Format("2006-01-02"))
		}

		found := false

		for _, tick := range chain {
			var avgFillPrice float64

			if option.OptionType == eventmodels.OptionTypeCall {
				avgFillPrice = tick.Ask
			} else if option.OptionType == eventmodels.OptionTypePut {
				avgFillPrice = tick.Bid
			} else {
				return fmt.Errorf("addAdditionInfoToOptionsV3: invalid option type %s", option.OptionType)
			}

			exp, err := time.Parse("2006-01-02", string(option.ExpirationDate))
			if err != nil {
				return fmt.Errorf("addAdditionInfoToOptionsV3: failed to parse expiration date %s: %v", option.ExpirationDate, err)
			}

			exp, err = eventmodels.ConvertToMarketClose(exp)
			if err != nil {
				return fmt.Errorf("addAdditionInfoToOptionsV3: failed to convert expiration date to market close: %v", err)
			}

			if tick.OptionType == string(option.OptionType) && tick.Strike == option.Strike && tick.ContractSize == option.ContractSize {
				options[i].Timestamp = tick.Timestamp
				options[i].Symbol = eventmodels.OptionSymbol(tick.Symbol)
				options[i].Description = tick.Description
				options[i].ExpirationType = tick.ExpirationType
				options[i].Bid = tick.Bid
				options[i].Ask = tick.Ask
				options[i].AverageFillPrice = avgFillPrice
				options[i].Expiration = exp

				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("addAdditionInfoToOptionsV3: no option chain tick found for expiration %s", option.Expiration.Format("2006-01-02"))
		}
	}

	return nil
}

func addTickDataToOptionsV3(contracts []eventmodels.OptionContractV3, optionChainTicksByExpirationMap map[eventmodels.ExpirationDate][]*eventmodels.OptionChainTickDTO, polygonTickDataReq *eventmodels.PolygonTickDataRequest) error {
	for _, c := range contracts {
		url := fmt.Sprintf("%s/v2/aggs/ticker/%s/range/1/minute/%s/%s", polygonTickDataReq.BaseURL, c.Symbol, polygonTickDataReq.StartDate.Format("2006-01-02"), polygonTickDataReq.EndDate.Format("2006-01-02"))
		dtos, err := utils.FetchRecursively(url, FetchPolygonAggregateBars())
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

			if _, ok := optionChainTicksByExpirationMap[c.ExpirationDate]; !ok {
				optionChainTicksByExpirationMap[c.ExpirationDate] = make([]*eventmodels.OptionChainTickDTO, 0)
			}

			optionChainTicksByExpirationMap[c.ExpirationDate] = append(optionChainTicksByExpirationMap[c.ExpirationDate], &tick)
		}
	}

	return nil
}

func ConvertOptionsChain(ctx context.Context, symbol eventmodels.StockSymbol, filteredOptions []eventmodels.OptionContractV3, optionChainTicksByExpirationMap map[eventmodels.ExpirationDate][]*eventmodels.OptionChainTickDTO, polygonTickDataReq *eventmodels.PolygonTickDataRequest) ([]eventmodels.OptionContractV3, error) {
	tracer := otel.Tracer("FetchOptionChainWithParamsV3")
	_, span := tracer.Start(ctx, "FetchOptionChainWithParamsV3")
	defer span.End()

	if err := addTickDataToOptionsV3(filteredOptions, optionChainTicksByExpirationMap, polygonTickDataReq); err != nil {
		return nil, fmt.Errorf("failed to add tick data to options: %v", err)
	}

	if err := addAdditionInfoToOptionsV3(filteredOptions, optionChainTicksByExpirationMap); err != nil {
		return nil, fmt.Errorf("failed to add symbol name to options: %v", err)
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
