package marketdata

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/utils"
)

func FilterOptions(optionContracts map[time.Time][]models.OptionContractV3, baseStrikePrice float64, expirationInDays []int, optionTypes []models.OptionType, minDistanceBetweenStrikes float64, maxNoOfStrikes int, now time.Time) ([]time.Time, []models.OptionContractV3) {
	expirationDates, filteredOptions := filterOptionContractsV3(optionContracts, expirationInDays, optionTypes, maxNoOfStrikes, maxNoOfStrikes, minDistanceBetweenStrikes, baseStrikePrice, now)
	return expirationDates, filteredOptions
}

func addAdditionalInfoToOptionsV3(options []models.OptionContractV3, optionChainMap map[models.ExpirationDate]map[models.OptionType]map[float64][]*models.OptionChainTickDTO, now time.Time) ([]models.OptionContractV3, error) {
	var resultContracts []models.OptionContractV3

	for i, option := range options {
		expirationMap, ok := optionChainMap[option.ExpirationDate]
		if !ok {
			log.Debugf("addAdditionalInfoToOptionsV3: no tick data for expiration %s", option.Expiration.Format("2006-01-02"))
			continue
		}

		optionTypeMap, ok := expirationMap[option.OptionType]
		if !ok {
			log.Debugf("addAdditionalInfoToOptionsV3: no tick data for %s %s", option.Expiration.Format("2006-01-02"), option.OptionType)
			continue
		}

		chain, ok := optionTypeMap[option.Strike]
		if !ok {
			log.Debugf("addAdditionalInfoToOptionsV3: no tick data for %s %s strike=%.2f", option.Expiration.Format("2006-01-02"), option.OptionType, option.Strike)
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
			log.Debugf("addAdditionInfoToOptionsHistoricalV3: no option chain tick found for expiration %s, type %s, strike %f before the target timestamp %v. This can occur when the requested timestamp is before the start of the option chain data.", option.Expiration.Format("2006-01-02"), option.OptionType, option.Strike, now)
			continue
		}

		tick := chain[j-1] // Get the last element before the target

		exp, err := time.Parse("2006-01-02", string(option.ExpirationDate))
		if err != nil {
			log.Errorf("addAdditionInfoToOptionsHistoricalV3: failed to parse expiration date %s: %v", option.ExpirationDate, err)
			continue
		}

		exp, err = models.ConvertToMarketClose(exp)
		if err != nil {
			log.Errorf("addAdditionInfoToOptionsHistoricalV3: failed to convert expiration date to market close: %v", err)
			continue
		}

		var avgFillPrice float64

		switch option.OptionType {
		case models.OptionTypeCall:
			avgFillPrice = tick.Ask
		case models.OptionTypePut:
			avgFillPrice = tick.Bid
		default:
			log.Errorf("addAdditionInfoToOptionsHistoricalV3: invalid option type %s", option.OptionType)
			continue
		}

		options[i].Timestamp = tick.Timestamp
		options[i].Symbol = models.OptionSymbol(tick.Symbol)
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

func populateTickDataToOptionChainMap(contracts []models.OptionContractV3, optionChainTickMap map[models.ExpirationDate]map[models.OptionType]map[float64][]*models.OptionChainTickDTO, polygonTickDataReq *models.PolygonOptionTickDataRequest, cache *PolygonCache) error {
	log.Debugf("populateTickDataToOptionChainMap: start populating tick data to option chain map for %d contracts", len(contracts))

	const maxConcurrency = 3

	type contractResult struct {
		contract models.OptionContractV3
		dtos     *models.AggregateResult[models.PolygonAggregateBar]
	}

	var (
		mu      sync.Mutex
		results []contractResult
	)

	// Phase 1: Fetch minute bars for all contracts concurrently
	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(maxConcurrency)

	for _, c := range contracts {
		c := c
		g.Go(func() error {
			optionSymbol := models.OptionSymbol(c.Symbol)

			if cache != nil {
				if cached := cache.GetAggregateBars(optionSymbol, polygonTickDataReq.StartDate, polygonTickDataReq.EndDate, "1m"); cached != nil {
					mu.Lock()
					results = append(results, contractResult{contract: c, dtos: cached})
					mu.Unlock()
					return nil
				}
			}

			url := fmt.Sprintf("%s/v2/aggs/ticker/%s/range/1/minute/%s/%s", polygonTickDataReq.BaseURL, c.Symbol, polygonTickDataReq.StartDate.Format("2006-01-02"), polygonTickDataReq.EndDate.Format("2006-01-02"))
			isHistorical := true
			dtos, err := utils.FetchRecursively(url, polygonTickDataReq.ApiKey, FetchPolygonAggregateBars(isHistorical))
			if err != nil {
				log.Warnf("populateTickDataToOptionChainMap: failed to fetch minute bars for %v: %v", c.Symbol, err)
				return nil
			}

			// Only cache non-empty results; empty results should not be cached
			// so they get retried when the simulation time advances
			if cache != nil && len(dtos.Results) > 0 {
				cache.SetAggregateBars(optionSymbol, polygonTickDataReq.StartDate, polygonTickDataReq.EndDate, "1m", dtos)
			}

			mu.Lock()
			results = append(results, contractResult{contract: c, dtos: dtos})
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("populateTickDataToOptionChainMap: parallel minute bar fetch failed: %w", err)
	}

	// Phase 2: Identify contracts with empty minute bars, retry with widened date range
	var emptyContracts []models.OptionContractV3
	minuteBarCount := 0

	for _, r := range results {
		if len(r.dtos.Results) == 0 {
			emptyContracts = append(emptyContracts, r.contract)
		} else {
			minuteBarCount++
		}
	}

	// For contracts with no minute bars in the narrow window, retry with a wider
	// range: StartDate to contract expiration. This catches contracts that haven't
	// started actively trading yet at the current simulation time.
	if len(emptyContracts) > 0 {
		g2, _ := errgroup.WithContext(context.Background())
		g2.SetLimit(maxConcurrency)

		for _, c := range emptyContracts {
			c := c
			g2.Go(func() error {
				// Parse expiration date to use as the wide end date
				expTime, err := time.Parse("2006-01-02", string(c.ExpirationDate))
				if err != nil {
					log.Debugf("populateTickDataToOptionChainMap: failed to parse expiration date for %v: %v", c.Symbol, err)
					return nil
				}

				optionSymbol := models.OptionSymbol(c.Symbol)
				wideEndDate := expTime

				// Use a "wide" cache key so it doesn't collide with the narrow fetch
				if cache != nil {
					if cached := cache.GetAggregateBars(optionSymbol, polygonTickDataReq.StartDate, wideEndDate, "1m-wide"); cached != nil {
						if len(cached.Results) > 0 {
							mu.Lock()
							results = append(results, contractResult{contract: c, dtos: cached})
							mu.Unlock()
						}
						return nil
					}
				}

				url := fmt.Sprintf("%s/v2/aggs/ticker/%s/range/1/minute/%s/%s", polygonTickDataReq.BaseURL, c.Symbol, polygonTickDataReq.StartDate.Format("2006-01-02"), wideEndDate.Format("2006-01-02"))
				isHistorical := true
				dtos, err := utils.FetchRecursively(url, polygonTickDataReq.ApiKey, FetchPolygonAggregateBars(isHistorical))
				if err != nil {
					log.Debugf("populateTickDataToOptionChainMap: failed to fetch wide minute bars for %v: %v", c.Symbol, err)
					return nil
				}

				// Only cache non-empty results; empty results should be retried
				// on subsequent calls as the simulation time advances
				if cache != nil && len(dtos.Results) > 0 {
					cache.SetAggregateBars(optionSymbol, polygonTickDataReq.StartDate, wideEndDate, "1m-wide", dtos)
				}

				if len(dtos.Results) > 0 {
					mu.Lock()
					results = append(results, contractResult{contract: c, dtos: dtos})
					mu.Unlock()
				}

				return nil
			})
		}

		if err := g2.Wait(); err != nil {
			return fmt.Errorf("populateTickDataToOptionChainMap: parallel wide minute bar fetch failed: %w", err)
		}

		// Recount after wide fetch
		minuteBarCount = 0
		emptyContracts = nil
		for _, r := range results {
			if len(r.dtos.Results) == 0 {
				emptyContracts = append(emptyContracts, r.contract)
			} else {
				minuteBarCount++
			}
		}
	}

	// Phase 3: Populate tick map from minute bar results
	for _, r := range results {
		c := r.contract
		for _, dto := range r.dtos.Results {
			tick := models.OptionChainTickDTO{
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
				DataSource:     "minute_bar",
			}

			if _, ok := optionChainTickMap[c.ExpirationDate]; !ok {
				optionChainTickMap[c.ExpirationDate] = make(map[models.OptionType]map[float64][]*models.OptionChainTickDTO)
			}

			if _, ok := optionChainTickMap[c.ExpirationDate][c.OptionType]; !ok {
				optionChainTickMap[c.ExpirationDate][c.OptionType] = make(map[float64][]*models.OptionChainTickDTO)
			}

			if _, ok := optionChainTickMap[c.ExpirationDate][c.OptionType][c.Strike]; !ok {
				optionChainTickMap[c.ExpirationDate][c.OptionType][c.Strike] = make([]*models.OptionChainTickDTO, 0)
			}

			optionChainTickMap[c.ExpirationDate][c.OptionType][c.Strike] = append(optionChainTickMap[c.ExpirationDate][c.OptionType][c.Strike], &tick)
		}
	}

	// Sort minute bar ticks by timestamp
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

	log.Infof("populateTickDataToOptionChainMap: %d/%d contracts have minute bars, %d truly missing",
		minuteBarCount, len(contracts), len(emptyContracts))

	return nil
}

func makeOptionsChain(ctx context.Context, symbol models.StockSymbol, options []models.OptionContractV3, optionChainTicksByExpirationMap map[models.ExpirationDate]map[models.OptionType]map[float64][]*models.OptionChainTickDTO, polygonTickDataReq *models.PolygonOptionTickDataRequest, now time.Time, cache *PolygonCache) ([]models.OptionContractV3, error) {

	if err := populateTickDataToOptionChainMap(options, optionChainTicksByExpirationMap, polygonTickDataReq, cache); err != nil {
		return nil, fmt.Errorf("failed to add tick data to options: %v", err)
	}

	var filteredOptions []models.OptionContractV3
	var err error
	filteredOptions, err = addAdditionalInfoToOptionsV3(options, optionChainTicksByExpirationMap, now)
	if err != nil {
		return nil, fmt.Errorf("addAdditionInfoToOptionsHistoricalV3: failed to add symbol name to options: %v", err)
	}

	return filteredOptions, nil
}

func FetchOptionChainWithParamsV2(optionsByExpirationURL, optionChainURL, stockURL, bearerToken string, symbol models.StockSymbol, optionTypes []models.OptionType, expirationInDays []int, minDistanceBetweenStrikes float64, maxNoOfStrikes int) ([]models.OptionContractV1, *models.StockTickItemDTO, error) {
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

func FetchOptionChainWithParamsV1(requestID uuid.UUID, optionsByExpirationURL, optionChainURL, stockURL, bearerToken string, symbol models.StockSymbol, optionTypes []models.OptionType, expirationInDays []int, minDistanceBetweenStrikes float64, maxNoOfStrikes int) ([]models.OptionContractV1, error) {
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
