package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"github.com/gocarina/gocsv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	export_data "slack-trading/src/cmd/stats/export_data/run"
	"slack-trading/src/eventmodels"
	"slack-trading/src/utils"
)

func importCandles(inDir string) (eventmodels.TradingViewCandles, error) {
	f, err := os.Open(inDir)
	if err != nil {
		return eventmodels.TradingViewCandles{}, fmt.Errorf("error opening file: %v", err)
	}

	defer f.Close()

	r := csv.NewReader(f)

	var candlesDTO eventmodels.TradingViewCandlesDTO

	gocsv.UnmarshalCSV(r, &candlesDTO)

	candles := candlesDTO.ToModel()

	candlesSorted := utils.SortCandles(candles, time.Minute*720)

	if err := candlesSorted.Validate(); err != nil {
		return nil, fmt.Errorf("error validating candles: %v", err)
	}

	return candlesSorted, nil
}

type RunArgs struct {
	StartsAt time.Time
	EndsAt   time.Time
	GoEnv    string
}

var rootCmd = &cobra.Command{
	Use:   "main",
	Short: "Generates the supertrend_4h_1h_stoch_rsi_15m_up signal",
	Long: `This program creates a multi-timeframe signal using the following indicators:
1.) The 4h and 1h supertrend indicators
2.) The 15m stochastic RSI indicator

The signal is generated when the following conditions are met:
1.) The 15m stochastic RSI is oversold
2.) The 4h and 1h supertrend indicators are in an uptrend
	`,
	Run: func(cmd *cobra.Command, args []string) {
		goEnv, err := cmd.Flags().GetString("go-env")
		if err != nil {
			log.Fatalf("error getting go-env: %v", err)
		}

		timezone, err := cmd.Flags().GetString("timezone")
		if err != nil {
			log.Fatalf("error getting timezone: %v", err)
		}

		loc, err := time.LoadLocation(timezone)
		if err != nil {
			log.Fatalf("error loading location: %v", err)
		}

		startsAtStr, err := cmd.Flags().GetString("starts-at")
		if err != nil {
			log.Fatalf("error getting starts-at flag: %v", err)
		}

		startsAt, err := time.ParseInLocation("2006-01-02T15:04:05", startsAtStr, loc)
		if err != nil {
			log.Fatalf("error parsing starts-at flag: %v", err)
		}

		endsAtStr, err := cmd.Flags().GetString("ends-at")
		if err != nil {
			log.Fatalf("error getting ends-at flag: %v", err)
		}

		endsAt, err := time.ParseInLocation("2006-01-02T15:04:05", endsAtStr, loc)
		if err != nil {
			log.Fatalf("error parsing ends-at flag: %v", err)
		}

		if err := run(RunArgs{
			StartsAt: startsAt,
			EndsAt:   endsAt,
			GoEnv:    goEnv,
		}); err != nil {
			log.Fatalf("error running command: %v", err)
		}
	},
}

func main() {
	rootCmd.PersistentFlags().StringVarP(new(string), "starts-at", "s", "", "Start period for generating signals. This should be in the format 'YYYY-MM-DDTHH:MM:SS-ZZ:ZZ', e.g. '2024-05-01T09:30:00-5:00'. This flag is required.")
	rootCmd.PersistentFlags().StringVarP(new(string), "ends-at", "e", "", "End period for generating signals. This should be in the format 'YYYY-MM-DDTHH:MM:SS-ZZ:ZZ', e.g. '2024-05-01T09:30:00-5:00'. This flag is required.")
	rootCmd.PersistentFlags().StringVarP(new(string), "timezone", "t", "America/New_York", "Timezone for the start and end dates. This should be a golang standard timezone.")
	rootCmd.PersistentFlags().StringVar(new(string), "go-env", "development", "The go environment to run the command in.")

	rootCmd.MarkPersistentFlagRequired("starts-at")
	rootCmd.MarkPersistentFlagRequired("ends-at")

	cobra.CheckErr(rootCmd.Execute())
}

func run(args RunArgs) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		panic("missing PROJECTS_DIR environment variable")
	}

	log.Infof("running with args: %v", args)

	// import data
	data := make([]eventmodels.TradingViewCandles, 3)
	streamNames := []string{"candles-SPX-15", "candles-SPX-60", "candles-SPX-240"}
	for i, streamName := range streamNames {
		output, err := export_data.Run(export_data.RunArgs{
			InputStreamName: streamName,
			StartsAt:        args.StartsAt,
			EndsAt:          args.EndsAt,
			GoEnv:           args.GoEnv,
		})

		if err != nil {
			return fmt.Errorf("error exporting data for %v: %v", streamName, err)
		}

		data[i], err = importCandles(output.ExportedFilepath)
		if err != nil {
			return fmt.Errorf("error fetching candles for stream %v: %v", streamName, err)
		}
	}

	// process data
	var candles15 eventmodels.TradingViewCandles = data[0]
	var candles60 eventmodels.TradingViewCandles = data[1]
	var candles240 eventmodels.TradingViewCandles = data[2]

	log.Infof("processing %d 15m candles", len(candles15))

	signalCount := 0
	for i := 0; i < len(candles15)-1; i++ {
		c1 := candles15[i]
		c2 := candles15[i+1]

		if c1.K < c1.D && c2.K > c2.D && c1.D <= 20 {
			candle60 := candles60.FindClosestCandleBeforeOrAt(c2.Timestamp)
			candle240 := candles240.FindClosestCandleBeforeOrAt(c2.Timestamp)

			if candle60.UpTrend > 0 && candle240.UpTrend > 0 {
				c2.IsSignal = true
				signalCount += 1
			}
		}
	}

	log.Infof("found %d signals", signalCount)

	// Process the candles
	candleDuration := 15 * time.Minute
	lookaheadPeriods := []int{4, 8, 16, 24, 96, 192, 288, 480, 672}

	// export to csv
	utils.ExportToCsv(candles15, lookaheadPeriods, candleDuration, "candles-SPX-15")

	return nil
}
