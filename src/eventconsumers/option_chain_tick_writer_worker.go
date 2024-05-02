package eventconsumers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
	"slack-trading/src/eventservices"
)

type OptionChainTickWriterWorker struct {
	wg                *sync.WaitGroup
	stockQuotesURL    string
	optionChainURL    string
	brokerBearerToken string
	calendarURL       string
}

func NewOptionChainTickWriterWorker(wg *sync.WaitGroup, stockQuotesURL, optionChainURL, brokerBearerToken, calendarURL string) *OptionChainTickWriterWorker {
	return &OptionChainTickWriterWorker{
		wg:                wg,
		stockQuotesURL:    stockQuotesURL,
		optionChainURL:    optionChainURL,
		brokerBearerToken: brokerBearerToken,
		calendarURL:       calendarURL,
	}
}

func (w *OptionChainTickWriterWorker) run(ctx context.Context, optionContractsClient *OptionContractConsumer, trackerClient *TrackerConsumer) {
	defer w.wg.Done()

	ticker := time.NewTicker(20 * time.Second) // Adjust the duration as needed
	defer ticker.Stop()

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		fmt.Println(err)
		return
	}

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			nowEST := now.In(loc)
			nowUTC := now.UTC()

			payload, err := eventservices.FetchMarketCalendar(w.calendarURL, w.brokerBearerToken, nowUTC)
			if err != nil {
				log.Errorf("Failed to fetch market calendar: %v", err)
			}

			open, err := eventservices.IsMarketOpen(payload, nowEST)
			if err != nil {
				log.Errorf("Failed to check if market is open: %v", err)
			}

			if !open {
				log.Debug("Market is closed")
				continue
			}

			var ticks []*eventmodels.OptionChainTick

			// get real time stock symbols and option contracts
			allOptionContracts, allOptionContractsDone := optionContractsClient.GetSavedEvents()
			allTrackers, allTrackersDone := trackerClient.GetSavedEvents()

			stockSymbols, optionContracts, err := eventservices.GetCurrentStockAndOptionContracts(ctx, allOptionContracts, allTrackers)

			allOptionContractsDone()
			allTrackersDone()

			if err != nil {
				log.Errorf("Failed to get current stock and option contracts: %v", err)
				continue
			}

			// record stock ticks
			for _, symbol := range stockSymbols {
				stockTickDTO, err := eventservices.FetchStockTicks(symbol, w.stockQuotesURL, w.brokerBearerToken)
				if err == nil {
					stockTick := stockTickDTO.ToModel(uuid.New(), nowUTC)
					eventpubsub.PublishEvent("main", eventmodels.CreateNewStockTickEvent, stockTick)
				} else {
					log.Errorf("Failed to fetch stock ticks: %v", err)
				}
			}

			// record option contract ticks
			cache := map[string]*eventmodels.OptionChainTickDTO{}
			expirations := optionContracts.GetListOfExpirations()
			underlyingSymbols := optionContracts.GetListOfUnderlyingSymbols()
			for _, underlyingSymbol := range underlyingSymbols {
				for _, expiration := range expirations {
					ticksDTO, err := eventservices.FetchOptionContractTicks(w.optionChainURL, w.brokerBearerToken, underlyingSymbol, expiration)
					if err != nil {
						log.Errorf("Failed to fetch option contract ticks: %v", err)
						continue
					}

					for _, dto := range ticksDTO {
						cache[dto.Symbol] = dto
					}
				}
			}

			for _, optionContract := range optionContracts {
				dto, found := cache[string(optionContract.Symbol)]
				if !found {
					log.Errorf("Option contract %s not found in cache", optionContract.Symbol)
					continue
				}

				ticks = append(ticks, dto.ToModel(optionContract.GetMetaData().GetEventStreamID(), uuid.New(), nowUTC))
			}

			for _, tick := range ticks {
				eventpubsub.PublishEvent("main", eventmodels.CreateNewOptionChainTickEvent, tick)
			}

			log.Infof("Recorded %d option contract ticks", len(ticks))
		case <-ctx.Done():
			return
		}
	}
}

func (w *OptionChainTickWriterWorker) Start(ctx context.Context, optionContractsCli *OptionContractConsumer, trackersCli *TrackerConsumer) {
	w.wg.Add(1)

	go w.run(ctx, optionContractsCli, trackersCli)
}
