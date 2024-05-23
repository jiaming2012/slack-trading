package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gocarina/gocsv"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	"slack-trading/src/eventpubsub"
	"slack-trading/src/eventservices"
	"slack-trading/src/utils"
)

func parseMeta(fileName string) eventmodels.CsvMeta {
	// strip the symbol [ex. BATS_COIN, 5 (1).csv] from the filename
	// and use it as the symbol
	symbol := strings.Split(fileName, "_")[1]
	symbol = strings.Split(symbol, ".")[0]
	if idx := strings.Index(symbol, "("); idx > 0 {
		symbol = symbol[:idx]
	}
	symbol = strings.TrimSpace(symbol)
	components := strings.Split(symbol, ",")
	return eventmodels.CsvMeta{
		Symbol:    strings.TrimSpace(components[0]),
		Timeframe: strings.TrimSpace(components[1]),
	}
}

func getStreamNameSuffix(meta eventmodels.CsvMeta) string {
	return fmt.Sprintf("%s-%s", meta.Symbol, meta.Timeframe)
}

func isDifferent(c1, c2 *eventmodels.CsvCandle) bool {
	return c1.Open != c2.Open || c1.High != c2.High || c1.Low != c2.Low || c1.Close != c2.Close || c1.UpTrendBegins != c2.UpTrendBegins || c1.DownTrendBegins != c2.DownTrendBegins
}

func main() {
	ctx := context.Background()
	wg := sync.WaitGroup{}

	eventpubsub.Init()

	if err := utils.InitEnvironmentVariables(); err != nil {
		log.Fatal(fmt.Errorf("error initializing environment variables: %v", err))
	}

	eventStoreDBURL := os.Getenv("EVENTSTOREDB_URL")

	// set db connection
	esdbProducer := eventproducers.NewESDBProducer(&wg, eventStoreDBURL, []eventmodels.StreamParameter{})
	esdbProducer.Start(ctx, nil)

	// open files inside csv_data folder
	files, err := os.ReadDir("./csv_data")
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".csv" {
			var csvCandlesDTO []eventmodels.CsvCandleDTO

			csvMeta := parseMeta(file.Name())

			streamNameSuffix := getStreamNameSuffix(csvMeta)

			allCandles, err := eventservices.FetchAll(ctx, esdbProducer.GetClient(), eventmodels.NewCsvCandle(eventmodels.StreamName(fmt.Sprintf("candles-%s", streamNameSuffix)), eventmodels.CandleSavedEvent, 1))
			if err != nil {
				log.Warnf("error fetching all (this can happen if the stream doesn't exist yet): %v", err)
			}

			cache := make(map[time.Time]*eventmodels.CsvCandle)
			for _, c := range allCandles {
				cache[c.Timestamp] = c
			}

			f, err := os.Open(filepath.Join("./csv_data", file.Name()))
			if err != nil {
				log.Fatalf("error opening file: %v", err)
			}
			defer f.Close()

			// Parse the CSV file into CsvTick objects and append them to csvTicks
			if err := gocsv.UnmarshalFile(f, &csvCandlesDTO); err != nil { // Load clients from file
				log.Fatalf("error unmarshalling %s: %v", file.Name(), err)
			}

			// conver to model & set meta
			var candles []*eventmodels.CsvCandle
			for _, dto := range csvCandlesDTO {
				c := dto.ToModel()
				c.SavedEventParms = eventmodels.SavedEventParameters{
					StreamName:    eventmodels.StreamName(fmt.Sprintf("candles-%s", streamNameSuffix)),
					EventName:     eventmodels.CandleSavedEvent,
					SchemaVersion: 1,
				}

				candles = append(candles, c)
			}

			// save to db
			savedCount := 0
			for _, c := range candles {
				if cachedCandle, found := cache[c.Timestamp]; !found || isDifferent(c, cachedCandle) {
					if err := esdbProducer.Save(c); err != nil {
						log.Fatalf("error saving candle: %v", err)
					}

					savedCount += 1
				}
			}

			log.Infof("Saved %d candles for %s", savedCount, streamNameSuffix)
		}
	}
}
