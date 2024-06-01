package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"github.com/gocarina/gocsv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"slack-trading/src/eventmodels"
	"slack-trading/src/utils"
)

func fetchCandles(inDir string) (eventmodels.TradingViewCandles, error) {
	f, err := os.Open(inDir)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
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
	StartsAt string
	EndsAt   string
	Args     []string
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
		startsAt, err := cmd.Flags().GetString("starts_at")
		if err != nil {
			log.Fatalf("error getting starts_at flag: %v", err)
		}

		endsAt, err := cmd.Flags().GetString("ends_at")
		if err != nil {
			log.Fatalf("error getting ends_at flag: %v", err)
		}

		if err := run(RunArgs{
			StartsAt: startsAt,
			EndsAt:   endsAt,
			Args:     args,
		}); err != nil {
			log.Fatalf("error running command: %v", err)
		}
	},
}

func main() {
	rootCmd.PersistentFlags().StringVarP(new(string), "starts_at", "s", "", "Start period for generating signals. This should be in the format 'YYYY-MM-DDTHH:MM:SS-ZZ:ZZ', e.g. '2024-05-01T09:30:00-5:00'. This flag is required.")
	rootCmd.PersistentFlags().StringVarP(new(string), "ends_at", "e", "", "End period for generating signals. This should be in the format 'YYYY-MM-DDTHH:MM:SS-ZZ:ZZ', e.g. '2024-05-01T09:30:00-5:00'. This flag is required.")
	rootCmd.MarkPersistentFlagRequired("starts_at")
	rootCmd.MarkPersistentFlagRequired("ends_at")
	cobra.CheckErr(rootCmd.Execute())
}

func run(args RunArgs) error {
	fmt.Println("Hello, world!")
	fmt.Println("startsAt: ", args.StartsAt)
	fmt.Println("endsAt: ", args.EndsAt)
	fmt.Println("args: ", args.Args)

	return nil
}

// 	projectsDir := os.Getenv("PROJECTS_DIR")
// 	if projectsDir == "" {
// 		panic("missing PROJECTS_DIR environment variable")
// 	}

// 	// fetch 15m candles
// 	fName := "candles-SPX-15.csv"
// 	inDir := filepath.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "data", fName)
// 	candles15, err := fetchCandles(inDir)
// 	if err != nil {
// 		log.Fatalf("error fetching candles (tf=15): %v", err)
// 	}

// 	// fetch 1h candles
// 	fName = "candles-SPX-60.csv"
// 	inDir = filepath.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "data", fName)
// 	candles60, err := fetchCandles(inDir)
// 	if err != nil {
// 		log.Fatalf("error fetching candles (tf=60): %v", err)
// 	}

// 	// fetch 4h candles
// 	fName = "candles-SPX-240.csv"
// 	inDir = filepath.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "data", fName)
// 	candles240, err := fetchCandles(inDir)
// 	if err != nil {
// 		log.Fatalf("error fetching candles (tf=240): %v", err)
// 	}

// 	signalCount := 0
// 	for i := 0; i < len(candles15)-1; i++ {
// 		c1 := candles15[i]
// 		c2 := candles15[i+1]

// 		if c1.K < c1.D && c2.K > c2.D && c1.D <= 20 {
// 			candle60 := candles60.FindClosestCandleBeforeOrAt(c2.Timestamp)
// 			candle240 := candles240.FindClosestCandleBeforeOrAt(c2.Timestamp)

// 			if candle60.UpTrend > 0 && candle240.UpTrend > 0 {
// 				c2.IsSignal = true
// 				signalCount += 1
// 			}
// 		}
// 	}

// 	log.Infof("15m candles: %d", len(candles15))
// 	log.Infof("found %d signals", signalCount)

// 	// Process the candles
// 	candleDuration := 15 * time.Minute
// 	lookaheadPeriods := []int{4, 8, 16, 24, 96, 192, 288, 480, 672}

// 	// export to csv
// 	utils.ExportToCsv(candles15, lookaheadPeriods, candleDuration, "candles-SPX-15")
// }
