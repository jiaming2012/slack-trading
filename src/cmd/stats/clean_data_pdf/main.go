package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
	"slack-trading/src/eventservices"
	"slack-trading/src/utils"
)

// go run main.go SPX 15 "4,8,16,24,96,192,288,480,672"

func main() {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		panic("missing PROJECTS_DIR environment variable")
	}

	// input variables
	symbol := os.Args[1]
	timeframeStr := os.Args[2]

	timeframe, err := strconv.ParseInt(timeframeStr, 10, 64)
	if err != nil {
		log.Fatalf("error parsing timeframe: %v", err)
	}

	lookaheadPeriodsStr := os.Args[3]
	lookaheadPeriods, err := utils.ParseLookaheadPeriods(lookaheadPeriodsStr)
	if err != nil {
		log.Fatalf("error parsing lookahead periods: %v", err)
	}

	inputStream := fmt.Sprintf("candles-%s-%s", symbol, timeframeStr)

	ctx := context.Background()

	eventpubsub.Init()

	if err := utils.InitEnvironmentVariables(); err != nil {
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

	csvCandleInstance := eventmodels.NewCsvCandleDTO(eventmodels.StreamName(inputStream), eventmodels.CandleSavedEvent, 1)
	csvCandlesDTO, err := eventservices.FetchAll(ctx, esdbClient, csvCandleInstance)
	if err != nil {
		log.Fatalf("error fetching all candles: %v", err)
	}

	var csvCandles []*eventmodels.TradingViewCandle
	for _, csvCandlesDTO := range csvCandlesDTO {
		csvCandles = append(csvCandles, csvCandlesDTO.ToModel())
	}

	log.Infof("Fetched %d candles\n", len(csvCandles))

	// Process the candles
	candleDuration := time.Duration(timeframe) * time.Minute

	csvCandles = utils.SortCandles(csvCandles, candleDuration)
	for _, c := range csvCandles {
		c.IsSignal = true
	}

	outDir := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "clean_data_pdf", inputStream)

	utils.ExportToCsv(csvCandles, lookaheadPeriods, candleDuration, outDir)
}
