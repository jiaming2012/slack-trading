package eventservices

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
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

func ConvertOptionsChain(ctx context.Context, symbol eventmodels.StockSymbol, filteredOptions []eventmodels.OptionContractV3, optionChainTicksByExpirationMap map[eventmodels.ExpirationDate][]*eventmodels.OptionChainTickDTO) ([]eventmodels.OptionContractV3, error) {
	tracer := otel.Tracer("FetchOptionChainWithParamsV3")
	_, span := tracer.Start(ctx, "FetchOptionChainWithParamsV3")
	defer span.End()

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
