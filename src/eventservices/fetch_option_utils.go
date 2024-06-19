package eventservices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func findOptionContractsGroupedByExpirationV3(targetExpirationDate string, contractMap map[time.Time][]eventmodels.OptionContractV3) (time.Time, []eventmodels.OptionContractV3, error) {
	var closestContractExpDate time.Time = time.Time{}
	minDiff := int(^uint(0) >> 1) // Max int

	expDate, err := time.Parse("2006-01-02", targetExpirationDate)
	if err != nil {
		return time.Time{}, nil, err
	}

	for contractExpDate := range contractMap {
		daysUntilExpiration := int(contractExpDate.Sub(expDate).Hours() / 24)
		if daysUntilExpiration < 0 {
			daysUntilExpiration = -daysUntilExpiration
		}

		if daysUntilExpiration < minDiff {
			minDiff = daysUntilExpiration
			closestContractExpDate = contractExpDate
		}
	}

	if closestContractExpDate.IsZero() {
		return time.Time{}, nil, fmt.Errorf("no matching contract found")
	}

	return closestContractExpDate, contractMap[closestContractExpDate], nil
}

func findOptionContractsGroupedByExpiration(targetExpirationDate string, contractMap map[time.Time][]eventmodels.OptionContractV1) (time.Time, []eventmodels.OptionContractV1, error) {
	var closestContractExpDate time.Time = time.Time{}
	minDiff := int(^uint(0) >> 1) // Max int

	expDate, err := time.Parse("2006-01-02", targetExpirationDate)
	if err != nil {
		return time.Time{}, nil, err
	}

	for contractExpDate := range contractMap {
		daysUntilExpiration := int(contractExpDate.Sub(expDate).Hours() / 24)
		if daysUntilExpiration < 0 {
			daysUntilExpiration = -daysUntilExpiration
		}

		if daysUntilExpiration < minDiff {
			minDiff = daysUntilExpiration
			closestContractExpDate = contractExpDate
		}
	}

	if closestContractExpDate.IsZero() {
		return time.Time{}, nil, fmt.Errorf("no matching contract found")
	}

	return closestContractExpDate, contractMap[closestContractExpDate], nil
}

func addAdditionInfoToOptionsV3(options []eventmodels.OptionContractV3, optionChainMap map[eventmodels.ExpirationDate][]*eventmodels.OptionChainTickDTO) error {
	for i, option := range options {
		chain, ok := optionChainMap[option.ExpirationDate]
		if !ok {
			return fmt.Errorf("addAdditionInfoToOptionsV3: no option chain found for expiration %s", option.Expiration.Format("2006-01-02"))
		}

		found := false

		for _, tick := range chain {
			if tick.OptionType == string(option.OptionType) && tick.Strike == option.Strike && tick.ContractSize == option.ContractSize {
				options[i].Symbol = eventmodels.OptionSymbol(tick.Symbol)
				options[i].Description = tick.Description
				options[i].ExpirationType = tick.ExpirationType
				options[i].Bid = tick.Bid
				options[i].Ask = tick.Ask
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

func addAdditionInfoToOptionsV2(options []eventmodels.OptionContractV1, optionChainMap map[time.Time][]*eventmodels.OptionChainTickDTO) error {
	for i, option := range options {
		chain, ok := optionChainMap[option.Expiration]
		if !ok {
			return fmt.Errorf("addAdditionInfoToOptionsV2: no option chain found for expiration %s", option.Expiration.Format("2006-01-02"))
		}

		found := false

		for _, tick := range chain {
			if tick.OptionType == string(option.OptionType) && tick.Strike == option.Strike && tick.ContractSize == option.ContractSize {
				options[i].Symbol = eventmodels.OptionSymbol(tick.Symbol)
				options[i].Description = tick.Description
				options[i].ExpirationType = tick.ExpirationType
				options[i].Bid = tick.Bid
				options[i].Ask = tick.Ask
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("addAdditionInfoToOptionsV2: no option chain tick found for expiration %s", option.Expiration.Format("2006-01-02"))
		}
	}

	return nil
}

func addAdditionInfoToOptionsV1(requestID uuid.UUID, options []eventmodels.OptionContractV1, optionChainMap map[time.Time][]*eventmodels.OptionChainTickDTO) error {
	for i, option := range options {
		chain, ok := optionChainMap[option.Expiration]
		if !ok {
			return fmt.Errorf("addAdditionInfoToOptionsV1: no option chain found for expiration %s", option.Expiration.Format("2006-01-02"))
		}

		found := false

		for _, tick := range chain {
			if tick.OptionType == string(option.OptionType) && tick.Strike == option.Strike && tick.ContractSize == option.ContractSize {
				options[i].SetMetaData(&eventmodels.MetaData{RequestID: requestID})
				options[i].Symbol = eventmodels.OptionSymbol(tick.Symbol)
				options[i].Description = tick.Description
				options[i].ExpirationType = tick.ExpirationType
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("addAdditionInfoToOptionsV1: no option chain tick found for expiration %s", option.Expiration.Format("2006-01-02"))
		}
	}

	return nil
}

func fetchOptionChainsV3(url, bearerToken string, symbol eventmodels.StockSymbol, expirations []time.Time) (map[eventmodels.ExpirationDate][]*eventmodels.OptionChainTickDTO, error) {
	optionChainMapCh := make(map[eventmodels.ExpirationDate][]*eventmodels.OptionChainTickDTO)

	for _, expiration := range expirations {
		expirationStr := expiration.Format("2006-01-02")

		optionChainTickDTO, err := FetchOptionContractTicks(url, bearerToken, symbol, expirationStr)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch option chain tick: %v", err)
		}

		expirationDate := eventmodels.ExpirationDate(expirationStr)
		optionChainMapCh[expirationDate] = optionChainTickDTO
	}

	return optionChainMapCh, nil
}

func fetchOptionChains(url, bearerToken string, symbol eventmodels.StockSymbol, expirations []time.Time) (map[time.Time][]*eventmodels.OptionChainTickDTO, error) {
	optionChainMapCh := make(map[time.Time][]*eventmodels.OptionChainTickDTO)

	for _, expiration := range expirations {
		expirationStr := expiration.Format("2006-01-02")

		optionChainTickDTO, err := FetchOptionContractTicks(url, bearerToken, symbol, expirationStr)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch option chain tick: %v", err)
		}

		optionChainMapCh[expiration] = optionChainTickDTO
	}

	return optionChainMapCh, nil
}

func splitAndSortContractsByStrikeV3(contracts []eventmodels.OptionContractV3, strike float64) eventmodels.OptionLadderV3 {
	var ladder eventmodels.OptionLadderV3

	for _, c := range contracts {
		switch c.OptionType {
		case eventmodels.OptionTypeCall:
			if c.Strike < strike {
				ladder.CallsBelowStrike = append(ladder.CallsBelowStrike, c)
			} else {
				ladder.CallsAboveStrike = append(ladder.CallsAboveStrike, c)
			}
		case eventmodels.OptionTypePut:
			if c.Strike < strike {
				ladder.PutsBelowStrike = append(ladder.PutsBelowStrike, c)
			} else {
				ladder.PutsAboveStrike = append(ladder.PutsAboveStrike, c)
			}
		default:
			continue
		}
	}

	sort.Slice(ladder.CallsAboveStrike, func(i, j int) bool {
		return ladder.CallsAboveStrike[i].Strike < ladder.CallsAboveStrike[j].Strike
	})

	sort.Slice(ladder.CallsBelowStrike, func(i, j int) bool {
		return ladder.CallsBelowStrike[i].Strike > ladder.CallsBelowStrike[j].Strike
	})

	sort.Slice(ladder.PutsAboveStrike, func(i, j int) bool {
		return ladder.PutsAboveStrike[i].Strike < ladder.PutsAboveStrike[j].Strike
	})

	sort.Slice(ladder.PutsBelowStrike, func(i, j int) bool {
		return ladder.PutsBelowStrike[i].Strike > ladder.PutsBelowStrike[j].Strike
	})

	return ladder
}

func splitAndSortContractsByStrike(contracts []eventmodels.OptionContractV1, strike float64) eventmodels.OptionLadder {
	var ladder eventmodels.OptionLadder

	for _, c := range contracts {
		switch c.OptionType {
		case eventmodels.OptionTypeCall:
			if c.Strike < strike {
				ladder.CallsBelowStrike = append(ladder.CallsBelowStrike, c)
			} else {
				ladder.CallsAboveStrike = append(ladder.CallsAboveStrike, c)
			}
		case eventmodels.OptionTypePut:
			if c.Strike < strike {
				ladder.PutsBelowStrike = append(ladder.PutsBelowStrike, c)
			} else {
				ladder.PutsAboveStrike = append(ladder.PutsAboveStrike, c)
			}
		default:
			continue
		}
	}

	sort.Slice(ladder.CallsAboveStrike, func(i, j int) bool {
		return ladder.CallsAboveStrike[i].Strike < ladder.CallsAboveStrike[j].Strike
	})

	sort.Slice(ladder.CallsBelowStrike, func(i, j int) bool {
		return ladder.CallsBelowStrike[i].Strike > ladder.CallsBelowStrike[j].Strike
	})

	sort.Slice(ladder.PutsAboveStrike, func(i, j int) bool {
		return ladder.PutsAboveStrike[i].Strike < ladder.PutsAboveStrike[j].Strike
	})

	sort.Slice(ladder.PutsBelowStrike, func(i, j int) bool {
		return ladder.PutsBelowStrike[i].Strike > ladder.PutsBelowStrike[j].Strike
	})

	return ladder
}

func filterOptionContractsV3(contractMap map[time.Time][]eventmodels.OptionContractV3, expirationInDays []int, optionTypes []eventmodels.OptionType, maxStrikesAbove int, maxStrikesBelow int, minDistanceBetweenStrikes float64, underlyingStockPrice float64, now time.Time) ([]time.Time, []eventmodels.OptionContractV3) {
	allResults := make([]eventmodels.OptionContractV3, 0)
	var includeCalls, includePuts bool
	for _, optionType := range optionTypes {
		if optionType == eventmodels.OptionTypeCall {
			includeCalls = true
		} else if optionType == eventmodels.OptionTypePut {
			includePuts = true
		}
	}

	var expirationDates []time.Time
	for _, days := range expirationInDays {
		targetExpirationDate := now.AddDate(0, 0, days).Format("2006-01-02")

		contractsExpirationDate, contracts, err := findOptionContractsGroupedByExpirationV3(targetExpirationDate, contractMap)
		if err != nil {
			continue
		}

		expirationDates = append(expirationDates, contractsExpirationDate)

		callResults := make([]eventmodels.OptionContractV3, 0)
		putResults := make([]eventmodels.OptionContractV3, 0)
		var callStrikesAbove, callStrikesBelow, putStrikesAbove, putStrikesBelow int

		ladder := splitAndSortContractsByStrikeV3(contracts, underlyingStockPrice)

		if includeCalls {
			for _, c := range ladder.CallsBelowStrike {
				if callStrikesBelow == 0 {
					callResults = append(callResults, c)
					callStrikesBelow++
				} else if callStrikesBelow < maxStrikesBelow {
					if callResults[callStrikesBelow-1].Strike-c.Strike >= minDistanceBetweenStrikes {
						callResults = append(callResults, c)
						callStrikesBelow++
					}
				} else {
					break
				}
			}

			for _, c := range ladder.CallsAboveStrike {
				if callStrikesAbove == 0 {
					callResults = append(callResults, c)
					callStrikesAbove++
				} else if callStrikesAbove < maxStrikesAbove {
					if c.Strike-callResults[len(callResults)-1].Strike >= minDistanceBetweenStrikes {
						callResults = append(callResults, c)
						callStrikesAbove++
					}
				} else {
					break
				}
			}
		}

		if includePuts {
			for _, p := range ladder.PutsBelowStrike {
				if putStrikesBelow == 0 {
					putResults = append(putResults, p)
					putStrikesBelow++
				} else if putStrikesBelow < maxStrikesBelow {
					if putResults[putStrikesBelow-1].Strike-p.Strike >= minDistanceBetweenStrikes {
						putResults = append(putResults, p)
						putStrikesBelow++
					}
				} else {
					break
				}
			}

			for _, p := range ladder.PutsAboveStrike {
				if putStrikesAbove == 0 {
					putResults = append(putResults, p)
					putStrikesAbove++
				} else if putStrikesAbove < maxStrikesAbove {
					if p.Strike-putResults[len(putResults)-1].Strike >= minDistanceBetweenStrikes {
						putResults = append(putResults, p)
						putStrikesAbove++
					}
				} else {
					break
				}
			}
		}

		allResults = append(allResults, callResults...)
		allResults = append(allResults, putResults...)
	}

	return expirationDates, allResults
}

func filterOptionContracts(contractMap map[time.Time][]eventmodels.OptionContractV1, expirationInDays []int, optionTypes []eventmodels.OptionType, maxStrikesAbove int, maxStrikesBelow int, minDistanceBetweenStrikes float64, underlyingStockPrice float64, now time.Time) ([]time.Time, []eventmodels.OptionContractV1) {
	allResults := make([]eventmodels.OptionContractV1, 0)
	var includeCalls, includePuts bool
	for _, optionType := range optionTypes {
		if optionType == eventmodels.OptionTypeCall {
			includeCalls = true
		} else if optionType == eventmodels.OptionTypePut {
			includePuts = true
		}
	}

	var expirationDates []time.Time
	for _, days := range expirationInDays {
		targetExpirationDate := now.AddDate(0, 0, days).Format("2006-01-02")

		contractsExpirationDate, contracts, err := findOptionContractsGroupedByExpiration(targetExpirationDate, contractMap)
		if err != nil {
			continue
		}

		expirationDates = append(expirationDates, contractsExpirationDate)

		callResults := make([]eventmodels.OptionContractV1, 0)
		putResults := make([]eventmodels.OptionContractV1, 0)
		var callStrikesAbove, callStrikesBelow, putStrikesAbove, putStrikesBelow int

		ladder := splitAndSortContractsByStrike(contracts, underlyingStockPrice)

		if includeCalls {
			for _, c := range ladder.CallsBelowStrike {
				if callStrikesBelow == 0 {
					callResults = append(callResults, c)
					callStrikesBelow++
				} else if callStrikesBelow < maxStrikesBelow {
					if callResults[callStrikesBelow-1].Strike-c.Strike >= minDistanceBetweenStrikes {
						callResults = append(callResults, c)
						callStrikesBelow++
					}
				} else {
					break
				}
			}

			for _, c := range ladder.CallsAboveStrike {
				if callStrikesAbove == 0 {
					callResults = append(callResults, c)
					callStrikesAbove++
				} else if callStrikesAbove < maxStrikesAbove {
					if c.Strike-callResults[len(callResults)-1].Strike >= minDistanceBetweenStrikes {
						callResults = append(callResults, c)
						callStrikesAbove++
					}
				} else {
					break
				}
			}
		}

		if includePuts {
			for _, p := range ladder.PutsBelowStrike {
				if putStrikesBelow == 0 {
					putResults = append(putResults, p)
					putStrikesBelow++
				} else if putStrikesBelow < maxStrikesBelow {
					if putResults[putStrikesBelow-1].Strike-p.Strike >= minDistanceBetweenStrikes {
						putResults = append(putResults, p)
						putStrikesBelow++
					}
				} else {
					break
				}
			}

			for _, p := range ladder.PutsAboveStrike {
				if putStrikesAbove == 0 {
					putResults = append(putResults, p)
					putStrikesAbove++
				} else if putStrikesAbove < maxStrikesAbove {
					if p.Strike-putResults[len(putResults)-1].Strike >= minDistanceBetweenStrikes {
						putResults = append(putResults, p)
						putStrikesAbove++
					}
				} else {
					break
				}
			}
		}

		allResults = append(allResults, callResults...)
		allResults = append(allResults, putResults...)
	}

	return expirationDates, allResults
}

func fetchTradierHistoricalPrices(url string, bearerToken string, symbol eventmodels.StockSymbol, startDate time.Time, endDate time.Time) ([]*eventmodels.TradierCandleDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchTradierHistoricalPrices: failed to create request: %w", err)
	}

	startDateStr := startDate.Format("2006-01-02")
	endDateStr := endDate.Format("2006-01-02")

	q := req.URL.Query()
	q.Add("symbol", string(symbol))
	q.Add("interval", "daily")
	q.Add("start", startDateStr)
	q.Add("end", endDateStr)

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchTradierHistoricalPrices: failed to fetch historical prices: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchTradierHistoricalPrices: failed to fetch historical prices, http code %v", res.Status)
	}

	var candles []*eventmodels.TradierCandleDTO
	if err := json.NewDecoder(res.Body).Decode(&candles); err != nil {
		return nil, fmt.Errorf("fetchTradierHistoricalPrices: failed to decode json: %w", err)
	}

	return candles, nil
}

func fetchTradierOptionsByExpiration(url, bearerToken string, symbol eventmodels.StockSymbol) (*eventmodels.OptionContractDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchExpirations: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("symbol", string(symbol))
	q.Add("strikes", "true")
	q.Add("expirationType", "true")
	q.Add("contractSize", "true")
	q.Add("includeAllRoots", "true")

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchExpirations: failed to fetch option chain: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchExpirations: failed to fetch option chain, http code %v", res.Status)
	}

	var dto eventmodels.OptionContractDTO
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("fetchExpirations: failed to decode json: %w", err)
	}

	return &dto, nil
}
