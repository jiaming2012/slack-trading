package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jiaming2012/slack-trading/src/cmd/backtester/src/run"
	"github.com/jiaming2012/slack-trading/src/eventconsumers"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

// func runTicks() {
// 	req := eventmodels.ThetaDataHistOptionOHLCRequest{
// 		Root:       "AAPL",
// 		Right:      eventmodels.ThetaDataOptionTypeCall,
// 		Expiration: time.Date(2023, time.November, 3, 0, 0, 0, 0, time.UTC),
// 		Strike:     170.0,
// 		StartDate:  time.Date(2023, time.November, 3, 0, 0, 0, 0, time.UTC),
// 		EndDate:    time.Date(2023, time.November, 3, 0, 0, 0, 0, time.UTC),
// 		Interval:   1 * time.Minute,
// 	}

// 	baseURL := "http://localhost:25510"
// 	resp, err := eventservices.FetchHistOptionOHLC(baseURL, req)
// 	if err != nil {
// 		panic(fmt.Errorf("failed to fetch option ohlc: %w", err))
// 	}

// 	candlesDTO, err := resp.ToHistOptionOhlcDTO()
// 	if err != nil {
// 		panic(fmt.Errorf("failed to convert response to dto: %w", err))
// 	}

// 	loc, err := time.LoadLocation("America/New_York")
// 	if err != nil {
// 		panic(fmt.Errorf("failed to load location: %w", err))
// 	}

// 	candles, err := eventmodels.HistOptionOhlcDTOs(candlesDTO).ConvertToHistOptionOhlc(loc)
// 	if err != nil {
// 		panic(fmt.Errorf("failed to convert dto to candle: %w", err))
// 	}

// 	for i, candle := range candles {
// 		fmt.Printf("%d: %+v\n", i, candle)
// 	}
// }

type RunArgs struct {
	OutDir string
	Symbol eventmodels.StockSymbol
	StartAtEventNumber uint64
}

type RunResults struct {
	SuccessMsg string
}

var runCmd = &cobra.Command{
	Use:   "go run src/cmd/backtester/src/main.go --outDir results",
	Short: "Backtest option signals",
	Run: func(cmd *cobra.Command, args []string) {
		outDir, err := cmd.Flags().GetString("outDir")
		if err != nil {
			log.Fatalf("error getting outDir: %v", err)
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			log.Fatalf("error getting symbol: %v", err)
		}

		startAtEventNumber, err := cmd.Flags().GetUint64("start-at")
		if err != nil {
			log.Fatalf("error getting start-at: %v", err)
		}

		results, err := Run(RunArgs{
			OutDir: outDir,
			Symbol: eventmodels.NewStockSymbol(symbol),
			StartAtEventNumber: startAtEventNumber,
		})

		if err != nil {
			log.Fatalf("Error: %v", err)
		}

		log.Info(results.SuccessMsg)
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

	slackWebhookURL := os.Getenv("SLACK_OPTION_ALERTS_WEBHOOK_URL")
	if slackWebhookURL == "" {
		return RunResults{}, fmt.Errorf("missing SLACK_OPTION_ALERTS_WEBHOOK_URL environment variable")
	}

	// Start slack client
	slackClient := eventconsumers.NewSlackNotifierClient(&wg, slackWebhookURL)

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

	execResult := run.Exec(ctx, &wg, args.Symbol, optionsConfig, args.StartAtEventNumber, args.OutDir, projectsDir, goEnv)
	if execResult.Err != nil {
		if err := slackClient.SendMessage(fmt.Sprintf("[ERROR] Backtest failed: %v", execResult.Err)); err != nil {
			log.Errorf("failed to send slack error message: %v", err)
		}

		return RunResults{}, fmt.Errorf("failed to run backtest: %w", execResult.Err)
	}

	if err := slackClient.SendMessage(fmt.Sprintf("[SUCCESS] Backtest complete: %v", execResult.SuccessMsg)); err != nil {
		log.Errorf("failed to send slack success message: %v", err)
	}

	return RunResults{
		SuccessMsg: execResult.SuccessMsg,
	}, nil
}

func main() {
	runCmd.PersistentFlags().String("outDir", "", "The directory to write the output to.")
	runCmd.PersistentFlags().String("symbol", "", "The stock symbol to backtest.")
	runCmd.PersistentFlags().Uint64("start-at", 0, "The event number to start at.")

	runCmd.MarkPersistentFlagRequired("outDir")
	runCmd.MarkPersistentFlagRequired("symbol")

	runCmd.Execute()
}
