package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
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

	m := eventservices.NewPolygonTickDataMachine(polygonApiKey)
	timespan := eventmodels.PolygonTimespan{
		Multiplier: 15,
		Unit:       eventmodels.Minute,
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

	candles, err := m.FetchAggregateBars(eventmodels.StockSymbol("AAPL"), timespan, from, to)
	if err != nil {
		log.Fatalf("failed to fetch aggregate bars: %v", err)
	}

	candlesJSON, err := json.Marshal(candles)
	if err != nil {
		log.Fatalf("failed to marshal candles to JSON: %v", err)
	}

	indicators := "supertrend, stochrsi, moving_averages, lag_features, atr, stochrsi_cross_above_20, stochrsi_cross_below_80"

	// Split the indicators string into a slice
	indicatorsList := strings.Split(indicators, ",")

	// Trim the spaces from each element in the slice
	for i, indicator := range indicatorsList {
		indicatorsList[i] = strings.TrimSpace(indicator)
	}

	// Run create_indicators.py and pass candles as JSON via standard input
	pythonInterp := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "env", "bin", "python3")
	fileDir := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "create_indicators.py")
	cmdArgs := append([]string{fileDir, "--indicators"}, indicatorsList...)
	cmd := exec.Command(pythonInterp, cmdArgs...)
	cmd.Stdin = bytes.NewReader(candlesJSON)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err = cmd.Run()
	if err != nil {
		log.Fatalf("failed to run create_indicators.py: %v\n%s", err, out.String())
	}

	// Unmarshall the json output from create_indicators.py
	var data []eventmodels.AggregateBarIndicator
	if err = json.Unmarshal(out.Bytes(), &data); err != nil {
		log.Fatalf("failed to unmarshal JSON output from create_indicators.py: %v", err)
	}

	// Print the last candle with indicators
	fmt.Printf("Last Candle: %+v\n", data[len(data)-1])

	repo := models.NewBacktesterCandleRepository(eventmodels.StockSymbol("AAPL"), 15*time.Minute, candles, indicators)

	fmt.Printf("Current Candle: +%v", repo.GetCurrentCandle())
}
