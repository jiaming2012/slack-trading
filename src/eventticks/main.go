package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	"slack-trading/src/eventpubsub"
)

func createCoinOptionContractsLookup(contracts []eventmodels.OptionContract) map[string]eventmodels.OptionContractID {
	lookup := make(map[string]eventmodels.OptionContractID)
	for _, contract := range contracts {
		lookup[contract.Symbol] = contract.ID
	}
	return lookup
}

func fetchOptionContractTicks(url, bearerToken string, symbol string, expiration string) ([]*eventmodels.OptionChainTickDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchOptionContractTicks: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("symbol", symbol)
	q.Add("expiration", expiration)

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchOptionContractTicks: failed to fetch option chain: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchOptionContractTicks: failed to fetch option chain, http code %v", res.Status)
	}

	var dto eventmodels.OptionContractChainDTO
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("fetchOptionContractTicks: failed to decode json: %w", err)
	}

	return dto.Options.Values, nil
}

func fetchStockTicks(symbol, url, bearerToken string) (*eventmodels.StockTickItemDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchStockTicks: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("symbols", symbol)

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchStockTicks: failed to fetch stock tick: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchStockTicks: failed to fetch stock tick, http code %v", res.Status)
	}

	var dto eventmodels.StockTickDTO
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("fetchOptionContractTicks: failed to decode json: %w", err)
	}

	return &dto.Quotes.Tick, nil
}

var cachedPayload *MarketCalendar

type MarketCalendar struct {
	Calendar struct {
		Month int `json:"month"`
		Year  int `json:"year"`
		Days  struct {
			Day []struct {
				Date        string `json:"date"`
				Status      string `json:"status"`
				Description string `json:"description"`
				Premarket   struct {
					Start string `json:"start"`
					End   string `json:"end"`
				} `json:"premarket"`
				Open struct {
					Start string `json:"start"`
					End   string `json:"end"`
				} `json:"open"`
				Postmarket struct {
					Start string `json:"start"`
					End   string `json:"end"`
				} `json:"postmarket"`
			} `json:"day"`
		} `json:"days"`
	} `json:"calendar"`
}

func fetchMarketCalendar(url, bearerToken string, now time.Time) (*MarketCalendar, error) {
	currentMonth := now.Format("2006-01")
	currentMonthInt, err := strconv.Atoi(currentMonth[5:])
	if err != nil {
		return nil, fmt.Errorf("fetchMarketCalendar: failed to parse current month: %w", err)
	}

	if cachedPayload != nil && cachedPayload.Calendar.Month == currentMonthInt {
		return cachedPayload, nil
	}

	log.Debugf("Cache invalid. Fetching market calendar for %v", currentMonth)

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchMarketCalendar: failed to create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchMarketCalendar: failed to fetch market calendar: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchMarketCalendar: failed to fetch market calendar, http code %v", res.Status)
	}

	var dto MarketCalendar
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("fetchMarketCalendar: failed to decode json: %w", err)
	}

	cachedPayload = &dto

	return &dto, nil
}

func isMarketOpen(calendar *MarketCalendar, now time.Time) (bool, error) {
	dateStr := now.Format("2006-01-02")
	timeStr := now.Format("15:04")

	for _, day := range calendar.Calendar.Days.Day {
		if day.Date == dateStr {
			if day.Status == "open" {
				start, err := time.Parse("15:04", day.Open.Start)
				if err != nil {
					return false, err
				}
				end, err := time.Parse("15:04", day.Open.End)
				if err != nil {
					return false, err
				}
				currentTime, err := time.Parse("15:04", timeStr)
				if err != nil {
					return false, err
				}

				if currentTime.After(start) && currentTime.Before(end) {
					return true, nil
				}
			}
			break
		}
	}

	return false, nil
}

func main() {
	ctx := context.Background()
	wg := sync.WaitGroup{}

	// Set up
	eventmodels.InitializeGlobalDispatcher()
	eventpubsub.Init()

	stockURL := "https://sandbox.tradier.com/v1/markets/quotes"
	calendarURL := "https://sandbox.tradier.com/v1/markets/calendar"
	optionChainURL := "https://sandbox.tradier.com/v1/markets/options/chains"
	brokerBearerToken := os.Getenv("TRADIER_BEARER_TOKEN")

	iDMap := createCoinOptionContractsLookup(eventmodels.CoinOptionContracts)

	streamParams := []eventmodels.StreamParameter{
		// {StreamName: eventmodels.AccountsStreamName, Mutex: &sync.Mutex{}},
		// {StreamName: eventmodels.OptionAlertsStreamName, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.OptionChainTickStream, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.StockTickStream, Mutex: &sync.Mutex{}},
	}

	eventproducers.NewEventStoreDBClient(&wg, streamParams).Start(ctx, os.Getenv("EVENTSTOREDB_URL"))

	ticker := time.NewTicker(20 * time.Second) // Adjust the duration as needed
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			payload, err := fetchMarketCalendar(calendarURL, brokerBearerToken, now)
			if err != nil {
				log.Errorf("Failed to fetch market calendar: %v", err)
			}

			open, err := isMarketOpen(payload, now)
			if err != nil {
				log.Errorf("Failed to check if market is open: %v", err)
			}

			if !open {
				log.Debug("Market is closed")
				continue
			}

			var ticks []*eventmodels.OptionChainTick

			// record stock ticks
			stockTickDTO, err := fetchStockTicks("coin", stockURL, brokerBearerToken)
			if err == nil {
				stockTick := stockTickDTO.ToModel(uuid.New(), now)
				eventpubsub.PublishEvent("main", eventmodels.CreateNewStockTickEvent, stockTick)
			} else {
				log.Errorf("Failed to fetch stock ticks: %v", err)
			}

			// record option contract ticks
			for _, expiration := range []string{"2024-04-12", "2024-04-19", "2024-05-17"} {
				ticksDTO, err := fetchOptionContractTicks(optionChainURL, brokerBearerToken, "coin", expiration)
				if err != nil {
					log.Errorf("Failed to fetch option contract ticks: %v", err)
					continue
				}

				for _, dto := range ticksDTO {
					contractID, found := iDMap[dto.Symbol]
					if !found {
						continue
					}

					ticks = append(ticks, dto.ToModel(contractID, uuid.New(), now))
				}
			}

			for _, tick := range ticks {
				eventpubsub.PublishEvent("main", eventmodels.CreateNewOptionChainTickEvent, tick)
			}

			log.Infof("Recorded %d option contract ticks\n", len(ticks))
		case <-ctx.Done():
			return
		}
	}
}
