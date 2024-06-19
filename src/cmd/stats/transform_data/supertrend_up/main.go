package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/utils"

	"github.com/jiaming2012/slack-trading/src/eventservices"

	"github.com/jiaming2012/slack-trading/src/eventpubsub"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func isUptrend(candle *eventmodels.TradingViewCandle) bool {
	if candle.UpTrend > 0 {
		return true
	} else if candle.DownTrend > 0 {
		return false
	}

	log.Fatalf("Invalid trend value: %v", candle)
	return false
}

func main() {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		panic("missing PROJECTS_DIR environment variable")
	}

	goEnv := os.Getenv("GO_ENV")
	if goEnv == "" {
		panic("missing GO_ENV environment variable")
	}

	// input variables
	inputStream := "candles-COIN-5"

	ctx := context.Background()

	eventpubsub.Init()

	if err := utils.InitEnvironmentVariables(projectsDir, goEnv); err != nil {
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
	allCandlesDTO, err := eventservices.FetchAll(ctx, esdbClient, csvCandleInstance)
	if err != nil {
		log.Fatalf("error fetching all candles: %v", err)
	}

	var allCandles []*eventmodels.TradingViewCandle
	for _, dto := range allCandlesDTO {
		c, err := dto.ToModel()
		if err != nil {
			log.Fatalf("error converting dto to model: %v", err)
		}

		allCandles = append(allCandles, c)
	}

	log.Infof("Fetched %d candles\n", len(allCandles))

	// Process the candles
	candleDuration := 5 * time.Minute
	lookaheadPeriods := []int{3, 10, 20, 40, 60, 120, 240, 1440, 2880}

	allCandles = utils.SortCandlesAndMarkSignals(allCandles, candleDuration, isUptrend)

	// export to csv
	streamName := fmt.Sprintf("candles-%s-5", "COIN")
	// fname := fmt.Sprintf("%s-from-%s-to-%s", streamName, args.StartsAt.Format("20060102_150405"), args.EndsAt.Format("20060102_150405"))
	fname := streamName
	outDir := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "transform_data", "supertrend_up", "output")
	utils.ExportToCsv(allCandles, lookaheadPeriods, candleDuration, outDir, fname)
}
