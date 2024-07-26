package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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

type RunArgs struct {
	OutDir     string
}

type RunResults struct {}

var runCmd = &cobra.Command{
	Use:   "go run src/cmd/backtester/src/main.go --outDir results",
	Short: "Backtest option signals",
	Run: func(cmd *cobra.Command, args []string) {
		outDir, err := cmd.Flags().GetString("outDir")
		if err != nil {
			log.Fatalf("error getting outDir: %v", err)
		}

		_, err = Run(RunArgs{
			OutDir: outDir,
		})

		if err != nil {
			log.Fatalf("Error: %v", err)
		}

		log.Info("Done")
	},
}

func Run(args RunArgs) (RunResults, error) {
	ctx := context.Background()
	wg := sync.WaitGroup{}
	goEnv := "development"

	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return RunResults{}, fmt.Errorf("missing PROJECTS_DIR environment variable")
	}

	if err := utils.InitEnvironmentVariables(projectsDir, goEnv); err != nil {
		return RunResults{}, fmt.Errorf("failed to init environment variables: %w", err)
	}

	// Load config
	optionsConfigInDir := path.Join(projectsDir, "slack-trading", "src", "options-config.yaml")
	data, err := os.ReadFile(optionsConfigInDir)
	if err != nil {
		return RunResults{}, fmt.Errorf("failed to read options config: %v", err)
	}

	var optionsConfig eventmodels.OptionsConfigYAML
	if err := yaml.Unmarshal(data, &optionsConfig); err != nil {
		return RunResults{}, fmt.Errorf("failed to unmarshal options config: %v", err)
	}

	run.Exec(ctx, &wg, optionsConfig, args.OutDir, goEnv)

	wg.Wait()

	return RunResults{}, nil
}

func main() {
	runCmd.PersistentFlags().String("outDir", "", "The directory to write the output to.")
	runCmd.MarkPersistentFlagRequired("outDir")
	runCmd.Execute()
}
