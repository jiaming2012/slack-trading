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

func (w *OptionChainTickWriterWorker) run(ctx context.Context, stockSymbols []eventmodels.StockSymbol, optionContracts eventmodels.OptionContracts) {
	defer w.wg.Done()

	ticker := time.NewTicker(10 * time.Second) // Adjust the duration as needed
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

			payload, err := eventservices.FetchMarketCalendar(w.calendarURL, w.brokerBearerToken, nowEST)
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
			expirations := optionContracts.GetListOfExpirations()
			for _, optionContract := range optionContracts {
				for _, expiration := range expirations {
					ticksDTO, err := eventservices.FetchOptionContractTicks(w.optionChainURL, w.brokerBearerToken, optionContract.UnderlyingSymbol, expiration)
					if err != nil {
						log.Errorf("Failed to fetch option contract ticks: %v", err)
						continue
					}

					// todo: make option symbol
					cache := map[string]*eventmodels.OptionChainTickDTO{}
					for _, dto := range ticksDTO {
						cache[dto.Symbol] = dto
					}

					for _, optionContract := range optionContracts {
						dto, found := cache[string(optionContract.Symbol)]
						if !found {
							continue
						}

						ticks = append(ticks, dto.ToModel(optionContract.GetMetaData().GetEventStreamID(), uuid.New(), nowUTC))
					}

					// for _, dto := range ticksDTO {
					// 	symbol := eventmodels.StockSymbol(dto.Symbol)
					// 	contractID, found := w.optionContractIDMap[symbol]
					// 	if !found {
					// 		continue
					// 	}

					// 	ticks = append(ticks, dto.ToModel(contractID, uuid.New(), nowUTC))
					// }
				}
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

// func (w *OptionChainTickWriterWorker) initializeOptionContractIDMap(contracts []*eventmodels.OptionContract) map[eventmodels.StockSymbol]eventmodels.EventStreamID {
// 	optionContractIDMap := make(map[eventmodels.StockSymbol]eventmodels.EventStreamID)

// 	for _, contract := range contracts {
// 		optionContractIDMap[contract.Symbol] = contract.Meta.EventStreamID
// 	}

// 	return optionContractIDMap
// }

func (w *OptionChainTickWriterWorker) Start(ctx context.Context, currentStockSymbols []eventmodels.StockSymbol, currentOptionContracts []*eventmodels.OptionContract) {
	w.wg.Add(1)

	// w.optionContractIDMap = w.initializeOptionContractIDMap(currentOptionContracts)

	go w.run(ctx, currentStockSymbols, currentOptionContracts)
}
