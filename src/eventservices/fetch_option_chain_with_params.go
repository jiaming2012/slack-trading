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

// if the data is live, then no tick will be greater than the current time stamp,
// instead, we will use the current time stamp to find the closest tick to the current time stamp
func addAdditionInfoToOptionsLive(options []eventmodels.OptionContractV3, optionChainMap map[eventmodels.ExpirationDate][]*eventmodels.OptionChainTickDTO, now time.Time) ([]eventmodels.OptionContractV3, error) {
	var resultContracts []eventmodels.OptionContractV3

	for i, option := range options {
		// todo: option map should perform look up by map with no iteration
		chain, ok := optionChainMap[option.ExpirationDate]
		if !ok {
			log.Errorf("addAdditionInfoToOptionsLive: no option chain found for expiration %s", option.ExpirationDate)
			continue
		}

		if len(chain) < 2 {
			log.Errorf("addAdditionInfoToOptionsLive: not enough option chain ticks for expiration %s", option.ExpirationDate)
			continue
		}

		found := false

		// todo: add check: is chain sorted by timestamp?

		for j := len(chain) - 1; j >= 0; j-- {
			if chain[j].Timestamp.After(now) {
				continue
			}

			var tick *eventmodels.OptionChainTickDTO
			if j == 0 {
				tick = chain[j]
			} else {
				tick = chain[j-1]
			}

			if tick.OptionType == string(option.OptionType) && tick.Strike == option.Strike && tick.ContractSize == option.ContractSize {
				exp, err := time.Parse("2006-01-02", string(option.ExpirationDate))
				if err != nil {
					log.Errorf("addAdditionInfoToOptionsLive: failed to parse expiration date %s: %v", option.ExpirationDate, err)
					continue
				}

				exp, err = eventmodels.ConvertToMarketClose(exp)
				if err != nil {
					log.Errorf("addAdditionInfoToOptionsLive: failed to convert expiration date to market close: %v", err)
					continue
				}

				var avgFillPrice float64

				if option.OptionType == eventmodels.OptionTypeCall {
					avgFillPrice = tick.Ask
				} else if option.OptionType == eventmodels.OptionTypePut {
					avgFillPrice = tick.Bid
				} else {
					log.Errorf("addAdditionInfoToOptionsLive: invalid option type %s", option.OptionType)
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

				found = true

				if tick.Timestamp.Sub(now) > 2*time.Hour {
					log.Warnf("addAdditionInfoToOptionsLive: %s datestamp %v that is more than 2 hours after the requested timestamp %v", options[i].Symbol, tick.Timestamp, now)
				}

				resultContracts = append(resultContracts, options[i])

				break
			}
		}

		if !found {
			log.Errorf("addAdditionInfoToOptionsLive: no option chain tick found for expiration %s", option.ExpirationDate)
		}
	}

	return resultContracts, nil
}

// if the data is historical, then we will use the current time stamp to find the closest tick to the current time stamp
func addAdditionInfoToOptionsHistoricalV3(options []eventmodels.OptionContractV3, optionChainMap map[eventmodels.ExpirationDate][]*eventmodels.OptionChainTickDTO, now time.Time) ([]eventmodels.OptionContractV3, error) {
	var resultContracts []eventmodels.OptionContractV3

	for i, option := range options {
		chain, ok := optionChainMap[option.ExpirationDate]
		if !ok {
			log.Errorf("addAdditionInfoToOptionsHistoricalV3: no option chain found for expiration %s", option.Expiration.Format("2006-01-02"))
			continue
		}

		if len(chain) < 2 {
			log.Errorf("addAdditionInfoToOptionsHistoricalV3: not enough option chain ticks for expiration %s", option.Expiration.Format("2006-01-02"))
			continue
		}

		found := false

		// todo: add check: is chain sorted by timestamp?

		for j := range chain {
			if chain[j].Timestamp.Before(now) {
				continue
			}

			if chain[j].Timestamp.Equal(now) {
				continue
			}

			var tick *eventmodels.OptionChainTickDTO
			if j == 0 {
				tick = chain[j]
			} else {
				tick = chain[j-1]
			}

			if tick.OptionType == string(option.OptionType) && tick.Strike == option.Strike && tick.ContractSize == option.ContractSize {
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

				if option.OptionType == eventmodels.OptionTypeCall {
					avgFillPrice = tick.Ask
				} else if option.OptionType == eventmodels.OptionTypePut {
					avgFillPrice = tick.Bid
				} else {
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

				found = true

				if tick.Timestamp.Sub(now) > 2*time.Hour {
					log.Warnf("addAdditionInfoToOptionsHistoricalV3: %s datestamp %v that is more than 2 hours after the requested timestamp %v", options[i].Symbol, tick.Timestamp, now)
				}

				resultContracts = append(resultContracts, options[i])

				break
			}
		}

		if !found {
			log.Errorf("addAdditionInfoToOptionsHistoricalV3: no option chain tick found for expiration %s", option.ExpirationDate)
		}
	}

	return resultContracts, nil
}

func addTickDataToOptionChainTicksByExpirationMap(contracts []eventmodels.OptionContractV3, optionChainTicksByExpirationMap map[eventmodels.ExpirationDate][]*eventmodels.OptionChainTickDTO, polygonTickDataReq *eventmodels.PolygonOptionTickDataRequest) error {
	for _, c := range contracts {
		url := fmt.Sprintf("%s/v2/aggs/ticker/%s/range/1/minute/%s/%s", polygonTickDataReq.BaseURL, c.Symbol, polygonTickDataReq.StartDate.Format("2006-01-02"), polygonTickDataReq.EndDate.Format("2006-01-02"))
		dtos, err := utils.FetchRecursively(url, polygonTickDataReq.ApiKey, FetchPolygonAggregateBars(polygonTickDataReq.IsHistorical))
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

func convertOptionsChain(ctx context.Context, symbol eventmodels.StockSymbol, options []eventmodels.OptionContractV3, optionChainTicksByExpirationMap map[eventmodels.ExpirationDate][]*eventmodels.OptionChainTickDTO, polygonTickDataReq *eventmodels.PolygonOptionTickDataRequest, now time.Time) ([]eventmodels.OptionContractV3, error) {
	tracer := otel.Tracer("FetchOptionChainWithParamsV3")
	_, span := tracer.Start(ctx, "FetchOptionChainWithParamsV3")
	defer span.End()

	if err := addTickDataToOptionChainTicksByExpirationMap(options, optionChainTicksByExpirationMap, polygonTickDataReq); err != nil {
		return nil, fmt.Errorf("failed to add tick data to options: %v", err)
	}

	var filteredOptions []eventmodels.OptionContractV3
	if polygonTickDataReq.IsHistorical {
		var err error
		filteredOptions, err = addAdditionInfoToOptionsHistoricalV3(options, optionChainTicksByExpirationMap, now)
		if err != nil {
			return nil, fmt.Errorf("addAdditionInfoToOptionsHistoricalV3: failed to add symbol name to options: %v", err)
		}
	} else {
		var err error
		filteredOptions, err = addAdditionInfoToOptionsLive(options, optionChainTicksByExpirationMap, now)
		if err != nil {
			return nil, fmt.Errorf("addAdditionInfoToOptionsLive: failed to add symbol name to options: %v", err)
		}
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
