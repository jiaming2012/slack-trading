package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/jiaming2012/slack-trading/src/cmd/backtester/src/run"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func runTicks() {
	req := eventmodels.ThetaDataHistOptionOHLCRequest{
		Root:       "AAPL",
		Right:      eventmodels.ThetaDataOptionTypeCall,
		Expiration: time.Date(2023, time.November, 3, 0, 0, 0, 0, time.UTC),
		Strike:     170.0,
		StartDate:  time.Date(2023, time.November, 3, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2023, time.November, 3, 0, 0, 0, 0, time.UTC),
		Interval:   1 * time.Minute,
	}

	baseURL := "http://localhost:25510"
	resp, err := eventservices.FetchHistOptionOHLC(baseURL, req)
	if err != nil {
		panic(fmt.Errorf("failed to fetch option ohlc: %w", err))
	}

	candlesDTO, err := resp.ToHistOptionOhlcDTO()
	if err != nil {
		panic(fmt.Errorf("failed to convert response to dto: %w", err))
	}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(fmt.Errorf("failed to load location: %w", err))
	}

	candles, err := eventmodels.HistOptionOhlcDTOs(candlesDTO).ConvertToHistOptionOhlc(loc)
	if err != nil {
		panic(fmt.Errorf("failed to convert dto to candle: %w", err))
	}

	for i, candle := range candles {
		fmt.Printf("%d: %+v\n", i, candle)
	}
}

func main() {
	ctx := context.Background()
	wg := sync.WaitGroup{}
	goEnv := "development"

	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		log.Fatalf("missing PROJECTS_DIR environment variable")
	}

	if err := utils.InitEnvironmentVariables(projectsDir, goEnv); err != nil {
		log.Panic(err)
	}

	// Load config
	optionsConfigInDir := path.Join(projectsDir, "slack-trading", "src", "options-config.yaml")
	data, err := os.ReadFile(optionsConfigInDir)
	if err != nil {
		log.Fatalf("failed to read options config: %v", err)
	}

	var optionsConfig eventmodels.OptionsConfigYAML
	if err := yaml.Unmarshal(data, &optionsConfig); err != nil {
		log.Fatalf("failed to unmarshal options config: %v", err)
	}

	run.Exec(ctx, &wg, optionsConfig, goEnv)

	wg.Wait()
}
