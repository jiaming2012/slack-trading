package main

import (
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"slack-trading/src/cmd/stats/export_data/run"
)

var rootCmd = &cobra.Command{
	Use:   "main",
	Short: "Export data from EventStoreDB to CSV",
	Long:  `This program exports data from EventStoreDB to CSV.`,
	Run: func(cmd *cobra.Command, args []string) {
		goEnv, err := cmd.Flags().GetString("go-env")
		if err != nil {
			log.Fatalf("error getting go-env: %v", err)
		}

		timeZone, err := cmd.Flags().GetString("timezone")
		if err != nil {
			log.Fatalf("error getting timezone: %v", err)
		}

		loc, err := time.LoadLocation(timeZone)
		if err != nil {
			log.Fatalf("error loading location: %v", err)
		}

		startsAtStr, err := cmd.Flags().GetString("starts-at")
		if err != nil {
			log.Fatalf("error getting starts-at: %v", err)
		}

		startsAt, err := time.ParseInLocation("2006-01-02T15:04:05", startsAtStr, loc)
		if err != nil {
			log.Fatalf("error parsing start date: %v", err)
		}

		endsAtStr, err := cmd.Flags().GetString("ends-at")
		if err != nil {
			log.Fatalf("error getting ends-at: %v", err)
		}

		endsAt, err := time.ParseInLocation("2006-01-02T15:04:05", endsAtStr, loc)
		if err != nil {
			log.Fatalf("error parsing end date: %v", err)
		}

		inputStreamName, err := cmd.Flags().GetString("stream-name")
		if err != nil {
			log.Fatalf("error getting stream_name: %v", err)
		}

		runArgs := run.RunArgs{
			StartsAt:        startsAt,
			EndsAt:          endsAt,
			InputStreamName: inputStreamName,
			GoEnv:           goEnv,
		}

		if _, err := run.Run(runArgs); err != nil {
			log.Fatalf("error running command: %v", err)
		}
	},
}

func main() {
	rootCmd.PersistentFlags().StringVarP(new(string), "starts-at", "s", "", "Start period for generating signals. This should be in the format 'YYYY-MM-DDTHH:MM:SS-ZZ:ZZ', e.g. '2024-05-01T09:30:00-5:00'. This flag is required.")
	rootCmd.PersistentFlags().StringVarP(new(string), "ends-at", "e", "", "End period for generating signals. This should be in the format 'YYYY-MM-DDTHH:MM:SS-ZZ:ZZ', e.g. '2024-05-01T09:30:00-5:00'. This flag is required.")
	rootCmd.PersistentFlags().StringVarP(new(string), "stream-name", "n", "", "The eventstore db stream name to export data from, e.g. candles-SPX-15. This flag is required.")
	rootCmd.PersistentFlags().StringVarP(new(string), "timezone", "t", "America/New_York", "Timezone for the start and end dates. This should be a golang standard timezone.")
	rootCmd.PersistentFlags().StringVar(new(string), "go-env", "development", "The go environment to run the command in.")

	rootCmd.MarkPersistentFlagRequired("start-at")
	rootCmd.MarkPersistentFlagRequired("ends-at")
	rootCmd.MarkPersistentFlagRequired("stream-name")

	cobra.CheckErr(rootCmd.Execute())
}
