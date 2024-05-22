package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
	"slack-trading/src/eventservices"
	"slack-trading/src/utils"
)

type StockData struct {
	Time          time.Time
	PercentChange float64
}

type Interval struct {
	LowPercentageChange  float64
	HighPercentageChange float64
}

func processCandlesPercentChange(candles []*eventmodels.CsvCandle, lookahead int) []StockData {
	stockData := make([]StockData, 0)
	for i := 0; i < len(candles)-lookahead; i++ {
		percentChange := (candles[i+lookahead].Close - candles[i].Close) / candles[i].Close * 100
		stockData = append(stockData, StockData{
			Time:          candles[i].Timestamp,
			PercentChange: percentChange,
		})
	}

	return stockData
}

func sortCandles(candles []*eventmodels.CsvCandle, timeFrameInMinutes time.Duration) []*eventmodels.CsvCandle {
	xValues := map[time.Time]*eventmodels.CsvCandle{}

	// remove duplicates
	for _, candle := range candles {
		xValues[candle.Timestamp] = candle
	}

	var candlesNoDuplicates []*eventmodels.CsvCandle
	for _, candle := range xValues {
		candlesNoDuplicates = append(candlesNoDuplicates, candle)
	}

	// sort candlesNoDuplicates by time
	sort.Slice(candlesNoDuplicates, func(i, j int) bool {
		return candlesNoDuplicates[i].Timestamp.Before(candlesNoDuplicates[j].Timestamp)
	})

	// check for gaps in the data
	for i := 0; i < len(candlesNoDuplicates)-1; i++ {
		if candlesNoDuplicates[i].Timestamp.Add(timeFrameInMinutes).Before(candlesNoDuplicates[i+1].Timestamp) {
			if !isMarkedClosed(candlesNoDuplicates[i]) {
				log.Warnf("Gap in data between %v and %v", candlesNoDuplicates[i].Timestamp, candlesNoDuplicates[i+1].Timestamp)
			}
		}
	}

	return candlesNoDuplicates
}

func isMarkedClosed(candle *eventmodels.CsvCandle) bool {
	// yes if between 19:55:00 +0000 UTC and 13:30:00 +0000 UTC
	// no otherwise
	if candle.Timestamp.Hour() == 19 && candle.Timestamp.Minute() >= 55 {
		return true
	}

	if candle.Timestamp.Hour() == 20 {
		return true
	}

	if candle.Timestamp.Hour() == 13 && candle.Timestamp.Minute() < 30 {
		return true
	}

	return false
}

func main() {
	// input variables
	inputStream := "candles-COIN-5"

	ctx := context.Background()

	eventpubsub.Init()

	pathToDevEnvFile := "../../../.env.development"
	pathToProdEnvFile := "../../../.env.production"
	if err := utils.InitEnvironmentVariables(pathToDevEnvFile, pathToProdEnvFile); err != nil {
		log.Fatalf("error initializing environment variables: %v", err)
	}

	settings, err := esdb.ParseConnectionString(os.Getenv("EVENTSTOREDB_URL"))
	if err != nil {
		log.Fatalf("error parsing connection string: %v", err)
	}

	esdbClient, err := esdb.NewClient(settings)
	if err != nil {
		log.Fatalf("error creating new client: %v", err)
	}

	csvCandleInstance := eventmodels.NewCsvCandle(eventmodels.StreamName(inputStream), eventmodels.CandleSavedEvent, 1)
	csvCandles, err := eventservices.FetchAll(ctx, esdbClient, csvCandleInstance)
	if err != nil {
		log.Fatalf("error fetching all candles: %v", err)
	}

	log.Infof("Fetched %d candles\n", len(csvCandles))

	// Process the candles
	candleDuration := 5 * time.Minute
	lookaheadPeriods := []int{3, 10, 20, 40, 60, 120, 240, 1440, 2880}

	csvCandles = sortCandles(csvCandles, candleDuration)

	for _, lookahead := range lookaheadPeriods {
		stockData := processCandlesPercentChange(csvCandles, lookahead)

		// Create export directory
		if _, err := os.Stat(inputStream); os.IsNotExist(err) {
			os.Mkdir(inputStream, 0755)
		}

		// Export the data
		lookaheadLabel := fmt.Sprintf("%d", lookahead*int(candleDuration.Minutes()))
		file, err := os.Create(fmt.Sprintf("%s/percent_change-%s.csv", inputStream, lookaheadLabel))
		if err != nil {
			fmt.Println("Error creating CSV file:", err)
			return
		}
		defer file.Close()

		writer := csv.NewWriter(file)
		defer writer.Flush()

		// Write header
		writer.Write([]string{"Time", "Percent_Change"})

		for _, data := range stockData {
			timeInISO := data.Time.Format(time.RFC3339)
			writer.Write([]string{timeInISO, fmt.Sprintf("%f", data.PercentChange)})
		}
	}
}
