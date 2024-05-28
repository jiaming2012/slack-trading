package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

func isDifferent(c1, c2 map[string]interface{}) bool {
	if len(c1) != len(c2) {
		return true
	}

	for k, v := range c1 {
		if v != c2[k] {
			return true
		}
	}

	return false
}

func main() {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		panic("missing PROJECTS_DIR environment variable")
	}

	ctx, cancel := context.WithCancel(context.Background())
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
			csvMeta := parseMeta(file.Name())

			streamNameSuffix := getStreamNameSuffix(csvMeta)

			allData, err := eventservices.FetchAllData(ctx, esdbProducer.GetClient(), eventmodels.NewCsvCandle(eventmodels.StreamName(fmt.Sprintf("candles-%s", streamNameSuffix)), eventmodels.CandleSavedEvent, 1))
			// allCandles, err := eventservices.FetchAll(ctx, esdbProducer.GetClient(), eventmodels.NewCsvCandle(eventmodels.StreamName(fmt.Sprintf("candles-%s", streamNameSuffix)), eventmodels.CandleSavedEvent, 1))
			if err != nil {
				log.Warnf("error fetching all (this can happen if the stream doesn't exist yet): %v", err)
			}

			cache := make(map[time.Time]map[string]interface{})
			for _, c := range allData {
				timestamp, err := time.Parse(time.RFC3339, c["time"].(string))
				if err != nil {
					log.Fatalf("error parsing time: %v", err)
				}

				cache[timestamp] = c
			}

			inDir := filepath.Join(projectsDir, "slack-trading", "src", "cmd", "import_data", "csv_data", file.Name())
			f, err := os.Open(inDir)
			if err != nil {
				log.Fatalf("error opening file: %v", err)
			}

			defer f.Close()

			r := csv.NewReader(f)

			records, err := r.ReadAll()
			if err != nil {
				log.Fatalf("error reading csv: %v", err)
			}

			log.Infof("Found %d records in %s", len(records), file.Name())

			// create a new event
			event := eventmodels.NewCsvCandle(eventmodels.StreamName(fmt.Sprintf("candles-%s", streamNameSuffix)), eventmodels.CandleSavedEvent, 1)

			// save to db
			savedCount := 0
			var headers []string
			for i, row := range records {
				if i == 0 {
					headers = append(headers, row...)
					continue
				}

				data := make(map[string]interface{})
				for j, v := range row {
					data[headers[j]] = v
				}

				timestamp, err := time.Parse(time.RFC3339, data["time"].(string))
				if err != nil {
					log.Fatalf("error parsing time: %v", err)
				}

				if cachedValue, found := cache[timestamp]; !found || isDifferent(data, cachedValue) {
					// delete any column that does not have a header
					delete(data, "")

					if err := esdbProducer.SaveData(event, data); err != nil {
						log.Fatalf("error saving candle: %v", err)
					}

					savedCount += 1
				}
			}

			log.Infof("Saved %d candles to %s", savedCount, streamNameSuffix)
		}
	}

	log.Info("Done saving candles")
	cancel()
}