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
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventproducers"
	"github.com/jiaming2012/slack-trading/src/eventpubsub"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func parseMeta(fileName string) eventmodels.CsvMeta {
	// strip the symbol [ex. BATS_COIN, 5 (1).csv] from the filename
	// and use it as the symbol
	components := strings.Split(fileName, ",")
	if len(components) > 2 {
		log.Fatalf("parseMeta: error parsing filename: %s", fileName)
	}

	symbolComponents := strings.Split(components[0], "_")
	if len(symbolComponents) < 2 {
		log.Fatalf("parseMeta: error parsing symbol: %s", components[0])
	}

	symbolStr := symbolComponents[len(symbolComponents)-1]

	timeframeComponents := strings.Split(components[1], ".")
	if len(timeframeComponents) < 2 {
		log.Fatalf("parseMeta: error parsing timeframe: %s", components[1])
	}

	timeframeStr := timeframeComponents[0]
	if idx := strings.Index(timeframeStr, "("); idx > 0 {
		timeframeStr = timeframeStr[:idx]
	}
	timeframeStr = strings.TrimSpace(timeframeStr)

	return eventmodels.CsvMeta{
		Symbol:    symbolStr,
		Timeframe: timeframeStr,
	}
}

type RunArgs struct {
	GoEnv      string
	StreamName eventmodels.StreamName
}

var rootCmd = &cobra.Command{
	Use:   "main",
	Short: "Import signals from trading view exported csv candles to EventStoreDB",
	Run: func(cmd *cobra.Command, args []string) {
		goEnv, err := cmd.Flags().GetString("go-env")
		if err != nil {
			log.Fatalf("error getting go-env: %v", err)
		}

		streamName, err := cmd.Flags().GetString("stream-name")
		if err != nil {
			log.Fatalf("error getting stream-name: %v", err)
		}

		if err := run(RunArgs{
			GoEnv:      goEnv,
			StreamName: eventmodels.StreamName(streamName),
		}); err != nil {
			log.Fatalf("error running command: %v", err)
		}
	},
}

func main() {
	rootCmd.PersistentFlags().StringVar(new(string), "go-env", "development", "The go environment to run the command in.")
	rootCmd.PersistentFlags().StringVar(new(string), "stream-name", "", "The stream name to export signals to.")

	rootCmd.MarkFlagRequired("stream-name")

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

func createSignal(ctx context.Context, signalName string, symbol eventmodels.StockSymbol, timestamp time.Time, timeframe uint, requestID uuid.UUID, esdbProducer *eventproducers.EsdbProducer) error {
	streamName := eventmodels.StreamName(fmt.Sprintf("backtest-signals-%s", symbol))
	header := eventmodels.NewSignalRequestHeader(timeframe, eventmodels.SignalSourceTradingView, symbol)
	event := eventmodels.NewSignalTrackerV3(signalName, *header, timestamp, requestID, streamName)
	return esdbProducer.SaveEvent(ctx, event)
}

func run(args RunArgs) error {
	// set up
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

	// set db connection
	eventStoreDBURL := os.Getenv("EVENTSTOREDB_URL")
	esdbProducer := eventproducers.NewESDBProducer(&wg, eventStoreDBURL, []eventmodels.StreamParameter{})
	esdbProducer.Start(ctx, nil)

	// open files inside csv_data folder
	baseDir := filepath.Join(projectsDir, "slack-trading", "src", "cmd", "import_signals", "csv_data")
	processedFilesBaseDir := filepath.Join(projectsDir, "slack-trading", "src", "cmd", "import_signals", "processed")
	files, err := os.ReadDir(baseDir)
	if err != nil {
		return fmt.Errorf("error reading directory: %v", err)
	}

	// check if all timeframes are present
	var symbol eventmodels.StockSymbol
	var found15, found60, found240 bool
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".csv" {
			csvMeta := parseMeta(file.Name())

			if symbol != "" && symbol != eventmodels.StockSymbol(csvMeta.Symbol) {
				log.Fatalf("Symbol mismatch: %s != %s", symbol, csvMeta.Symbol)
			}

			symbol = eventmodels.StockSymbol(csvMeta.Symbol)

			if csvMeta.Timeframe == "15" {
				found15 = true
			}

			if csvMeta.Timeframe == "60" {
				found60 = true
			}

			if csvMeta.Timeframe == "240" {
				found240 = true
			}
		}
	}

	if !found15 || !found60 || !found240 {
		log.Fatalf("Missing timeframes: 15=%t, 60=%t, 240=%t", found15, found60, found240)
	}

	// read csv files and import signals
	data := make(map[string]map[string]eventmodels.TradingViewCandleDTO)
	requestID := uuid.New()
	var timestamps []string

	// collect data
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".csv" {
			csvMeta := parseMeta(file.Name())

			inDir := filepath.Join(baseDir, file.Name())
			f, err := os.Open(inDir)
			if err != nil {
				return fmt.Errorf("error opening file: %v", err)
			}

			defer f.Close()

			var rows []*eventmodels.TradingViewCandleDTO
			if err := gocsv.UnmarshalFile(f, &rows); err != nil {
				return fmt.Errorf("error unmarshalling file: %v", err)
			}

			for _, row := range rows {
				if csvMeta.Timeframe == "15" {
					timestamps = append(timestamps, row.Timestamp)
				}

				if _, ok := data[row.Timestamp]; !ok {
					data[row.Timestamp] = make(map[string]eventmodels.TradingViewCandleDTO)
				}

				data[row.Timestamp][csvMeta.Timeframe] = *row
			}

			log.Infof("Saving %d rows for timeframe %s", len(rows), csvMeta.Timeframe)
		}
	}

	// export data as signals
	var prevCandle15 *eventmodels.TradingViewCandle
	for _, timestamp := range timestamps {
		candles240DTO, ok := data[timestamp]["240"]
		if ok {
			candles240, err := candles240DTO.ToModel()
			if err != nil {
				log.Panicf("Error converting 240 minute candles to model: %v", err)
			}

			if candles240.UpTrendBegins > 0 {
				if err := createSignal(ctx, "supertrend-buy", symbol, candles240.Timestamp, 240, requestID, esdbProducer); err != nil {
					log.Fatalf("Error creating signal: %v", err)
				}

				log.Infof("[240] Up trend begins at %s", timestamp)
			}

			if candles240.DownTrendBegins > 0 {
				if err := createSignal(ctx, "supertrend-sell", symbol, candles240.Timestamp, 240, requestID, esdbProducer); err != nil {
					log.Fatalf("Error creating signal: %v", err)
				}

				log.Infof("[240] Down trend begins at %s", timestamp)
			}
		}

		candle60DTO, ok := data[timestamp]["60"]
		if ok {
			candle60, err := candle60DTO.ToModel()
			if err != nil {
				log.Panicf("Error converting 60 minute candle to model: %v", err)
			}

			if candle60.UpTrendBegins > 0 {
				if err := createSignal(ctx, "supertrend-buy", symbol, candle60.Timestamp, 60, requestID, esdbProducer); err != nil {
					log.Fatalf("Error creating signal: %v", err)
				}

				log.Infof("[60] Up trend begins at %s", timestamp)
			}

			if candle60.DownTrendBegins > 0 {
				if err := createSignal(ctx, "supertrend-sell", symbol, candle60.Timestamp, 60, requestID, esdbProducer); err != nil {
					log.Fatalf("Error creating signal: %v", err)
				}

				log.Infof("[60] Down trend begins at %s", timestamp)
			}
		}

		candle15DTO, ok := data[timestamp]["15"]
		if !ok {
			log.Panicf("Missing 15 minute candle for timestamp %s", timestamp)
		}

		candle15, err := candle15DTO.ToModel()
		if err != nil {
			log.Panicf("Error converting 15 minute candle to model: %v", err)
		}

		if prevCandle15 != nil {
			c1 := candle15
			c2 := prevCandle15

			if c1.K > c1.D && c2.K < c2.D && c1.D >= 80 {
				if err := createSignal(ctx, "stochastic_rsi-sell", symbol, c1.Timestamp, 15, requestID, esdbProducer); err != nil {
					log.Fatalf("Error creating signal: %v", err)
				}

				log.Infof("Bearish divergence at %s", timestamp)
			}

			if c1.K < c1.D && c2.K > c2.D && c1.D <= 20 {
				if err := createSignal(ctx, "stochastic_rsi-buy", symbol, c1.Timestamp, 15, requestID, esdbProducer); err != nil {
					log.Fatalf("Error creating signal: %v", err)
				}

				log.Infof("Bullish divergence at %s", timestamp)
			}
		}

		prevCandle15 = candle15
	}

	log.Info("Finished importing signals")

	// move files to processed
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".csv" {
			currentFilePath := filepath.Join(baseDir, file.Name())
			if err := moveFileToProcessed(currentFilePath, processedFilesBaseDir); err != nil {
				log.Fatalf("error moving file to processed: %v", err)
			}

			log.Infof("Moved file %s to processed", file.Name())
		}
	}

	// clean up
	cancel()

	return nil
}
