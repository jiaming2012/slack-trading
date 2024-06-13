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
	"github.com/spf13/cobra"

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

type RunArgs struct {
	GoEnv string
}

var rootCmd = &cobra.Command{
	Use:   "main",
	Short: "Imports trading view exported csv candles to EventStoreDB",
	Run: func(cmd *cobra.Command, args []string) {
		goEnv, err := cmd.Flags().GetString("go-env")
		if err != nil {
			log.Fatalf("error getting go-env: %v", err)
		}

		if err := run(RunArgs{
			GoEnv: goEnv,
		}); err != nil {
			log.Fatalf("error running command: %v", err)
		}
	},
}

func main() {
	rootCmd.PersistentFlags().StringVar(new(string), "go-env", "development", "The go environment to run the command in.")

	cobra.CheckErr(rootCmd.Execute())
}

func moveFileToProcessed(currentFilePath, processedDir string) error {
	_, fileName := filepath.Split(currentFilePath)
	newFilePath := filepath.Join(processedDir, fileName)
	if err := os.Rename(currentFilePath, newFilePath); err != nil {
		return fmt.Errorf("error moving file to processed: %v", err)
	}

	return nil
}

func run(args RunArgs) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("missing PROJECTS_DIR environment variable")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := sync.WaitGroup{}

	eventpubsub.Init()

	if err := utils.InitEnvironmentVariables(projectsDir, args.GoEnv); err != nil {
		return fmt.Errorf("error initializing environment variables: %v", err)
	}

	eventStoreDBURL := os.Getenv("EVENTSTOREDB_URL")

	// set db connection
	esdbProducer := eventproducers.NewESDBProducer(&wg, eventStoreDBURL, []eventmodels.StreamParameter{})
	esdbProducer.Start(ctx, nil)

	// open files inside csv_data folder
	baseDir := filepath.Join(projectsDir, "slack-trading", "src", "cmd", "import_trading_view_data", "csv_data")
	processedFilesBaseDir := filepath.Join(projectsDir, "slack-trading", "src", "cmd", "import_trading_view_data", "processed")
	files, err := os.ReadDir(baseDir)
	if err != nil {
		return fmt.Errorf("error reading directory: %v", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".csv" {
			csvMeta := parseMeta(file.Name())

			streamNameSuffix := getStreamNameSuffix(csvMeta)

			allData, err := eventservices.FetchAllData(ctx, esdbProducer.GetClient(), eventmodels.NewCsvCandleDTO(eventmodels.StreamName(fmt.Sprintf("candles-%s", streamNameSuffix)), eventmodels.CandleSavedEvent, 1))
			if err != nil {
				log.Warnf("error fetching all (this can happen if the stream doesn't exist yet): %v", err)
			}

			cache := make(map[time.Time]map[string]interface{})
			for _, c := range allData {
				timestamp, err := time.Parse(time.RFC3339, c["time"].(string))
				if err != nil {
					return fmt.Errorf("error parsing time: %v", err)
				}

				cache[timestamp] = c
			}

			inDir := filepath.Join(baseDir, file.Name())
			f, err := os.Open(inDir)
			if err != nil {
				return fmt.Errorf("error opening file: %v", err)
			}

			defer f.Close()

			r := csv.NewReader(f)

			records, err := r.ReadAll()
			if err != nil {
				return fmt.Errorf("error reading csv: %v", err)
			}

			log.Infof("Found %d records in %s", len(records), file.Name())

			// create a new event
			event := eventmodels.NewCsvCandleDTO(eventmodels.StreamName(fmt.Sprintf("candles-%s", streamNameSuffix)), eventmodels.CandleSavedEvent, 1)

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
					return fmt.Errorf("error parsing time: %v", err)
				}

				if cachedValue, found := cache[timestamp]; !found || isDifferent(data, cachedValue) {
					// delete any column that does not have a header
					delete(data, "")

					if err := esdbProducer.SaveData(event, data); err != nil {
						return fmt.Errorf("error saving candle: %v", err)
					}

					savedCount += 1
				}
			}

			log.Infof("Saved %d candles to %s", savedCount, streamNameSuffix)

			if err := moveFileToProcessed(inDir, processedFilesBaseDir); err != nil {
				return fmt.Errorf("error moving file to processed: %v", err)
			}
		}
	}

	log.Info("Done saving candles")

	return nil
}
