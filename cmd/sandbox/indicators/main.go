package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func main() {
	goEnv := "development"

	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		log.Panicf("missing PROJECTS_DIR environment variable")
	}

	if err := utils.InitEnvironmentVariables(projectsDir, goEnv); err != nil {
		log.Panicf("failed to init environment variables")
	}

	polygonApiKey, err := utils.GetEnv("POLYGON_API_KEY")
	if err != nil {
		log.Fatalf("$POLYGON_API_KEY not set: %v", err)
	}

	m := eventservices.NewPolygonClient(polygonApiKey)
	timespan := eventmodels.PolygonTimespan{
		Multiplier: 15,
		Unit:       eventmodels.PolygonTimespanUnitMinute,
	}

	pastFrom := &eventmodels.PolygonDate{
		Year:  2021,
		Month: 7,
		Day:   15,
	}

	pastTo := &eventmodels.PolygonDate{
		Year:  2021,
		Month: 8,
		Day:   31,
	}

	from := &eventmodels.PolygonDate{
		Year:  2021,
		Month: 9,
		Day:   1,
	}

	to := &eventmodels.PolygonDate{
		Year:  2021,
		Month: 9,
		Day:   30,
	}

	pastCandlesForIndicators, err := m.FetchAggregateBars(eventmodels.StockSymbol("AAPL"), timespan, pastFrom, pastTo)
	if err != nil {
		log.Fatalf("failed to fetch past aggregate bars: %v", err)
	}

	candles, err := m.FetchAggregateBars(eventmodels.StockSymbol("AAPL"), timespan, from, to)
	if err != nil {
		log.Fatalf("failed to fetch aggregate bars: %v", err)
	}

	indicators := []string{"supertrend", "stochrsi", "moving_averages", "lag_features", "atr", "stochrsi_cross_above_20", "stochrsi_cross_below_80"}

	aggregateCandles := append(pastCandlesForIndicators, candles...)

	historyInDays := uint32(10)

	repoSource := eventmodels.CandleRepositorySource{
		Type: "polygon",
	}

	repo, err := models.NewCandleRepository(eventmodels.StockSymbol("AAPL"), 15*time.Minute, aggregateCandles, indicators, nil, historyInDays, repoSource)

	// Print the first candle
	fmt.Printf("First Candle: %+v\n", candles[0])

	candle, err := repo.GetCurrentCandle()

	fmt.Printf("Current Candle: +%v", candle)
}
