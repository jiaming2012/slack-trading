package main

import (
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"slack-trading/src/cmd/stats/transform_data/supertrend_4h_1h_stoch_rsi_15m_up/run"
	"slack-trading/src/eventmodels"
)

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

		ticker, err := cmd.Flags().GetString("ticker")
		if err != nil {
			log.Fatalf("error getting ticker flag: %v", err)
		}

		lookaheadCandlesCount, err := cmd.Flags().GetIntSlice("lookahead-candles-count")
		if err != nil {
			log.Fatalf("error getting lookahead-candles-count flag: %v", err)
		}

		if _, err := run.Run(run.RunArgs{
			StartsAt:              startsAt,
			EndsAt:                endsAt,
			GoEnv:                 goEnv,
			Ticker:                eventmodels.StockSymbol(ticker),
			LookaheadCandlesCount: lookaheadCandlesCount,
		}); err != nil {
			log.Fatalf("error running command: %v", err)
		}
	},
}

func main() {
	rootCmd.PersistentFlags().StringVarP(new(string), "ticker", "t", "", "Stock ticker to generate the signal for, e.g. 'SPX'. This flag is required.")
	rootCmd.PersistentFlags().StringVarP(new(string), "starts-at", "s", "", "Start period for generating signals. This should be in the format 'YYYY-MM-DDTHH:MM:SS-ZZ:ZZ', e.g. '2024-05-01T09:30:00-5:00'. This flag is required.")
	rootCmd.PersistentFlags().StringVarP(new(string), "ends-at", "e", "", "End period for generating signals. This should be in the format 'YYYY-MM-DDTHH:MM:SS-ZZ:ZZ', e.g. '2024-05-01T09:30:00-5:00'. This flag is required.")
	rootCmd.PersistentFlags().StringVarP(new(string), "timezone", "z", "America/New_York", "Timezone for the start and end dates. This should be a golang standard timezone.")
	rootCmd.PersistentFlags().StringVar(new(string), "go-env", "development", "The go environment to run the command in.")
	rootCmd.PersistentFlags().IntSliceVarP(new([]int), "lookahead-candles-count", "l", nil, "The number of 15 minute candles to look ahead to calculate the percent change. This should be a comma-separated list of integers.")

	rootCmd.MarkPersistentFlagRequired("ticker")
	rootCmd.MarkPersistentFlagRequired("starts-at")
	rootCmd.MarkPersistentFlagRequired("ends-at")
	rootCmd.MarkPersistentFlagRequired("lookahead-candles-count")

	cobra.CheckErr(rootCmd.Execute())
}
