package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path"
	"reflect"
	"time"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"slack-trading/src/cmd/stats/import_data/helpers"
	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
	"slack-trading/src/eventservices"
	"slack-trading/src/utils"
)

type RunArgs struct {
	InputStreamName string
	StartsAt        string
	EndsAt          string
	TimeZone        string
	GoEnv           string
}

var rootCmd = &cobra.Command{
	Use:   "main",
	Short: "Export data from EventStoreDB to CSV",
	Long:  `This program exports data from EventStoreDB to CSV.`,
	Run: func(cmd *cobra.Command, args []string) {
		goEnv, err := cmd.Flags().GetString("go-env")
		if err != nil {
			log.Fatalf("error getting go-env: %v", err)
		}

		startsAt, err := cmd.Flags().GetString("starts-at")
		if err != nil {
			log.Fatalf("error getting starts-at: %v", err)
		}

		endsAt, err := cmd.Flags().GetString("ends-at")
		if err != nil {
			log.Fatalf("error getting ends-at: %v", err)
		}

		timeZone, err := cmd.Flags().GetString("timezone")
		if err != nil {
			log.Fatalf("error getting timezone: %v", err)
		}

		inputStreamName, err := cmd.Flags().GetString("stream-name")
		if err != nil {
			log.Fatalf("error getting stream_name: %v", err)
		}

		runArgs := RunArgs{
			StartsAt:        startsAt,
			EndsAt:          endsAt,
			TimeZone:        timeZone,
			InputStreamName: inputStreamName,
			GoEnv:           goEnv,
		}

		if err := run(runArgs); err != nil {
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

func run(args RunArgs) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		panic("missing PROJECTS_DIR environment variable")
	}

	log.Infof("Running import_data_est using %v", args)

	loc, err := time.LoadLocation(args.TimeZone)
	if err != nil {
		return fmt.Errorf("error loading location: %v", err)
	}

	startsAt, err := time.ParseInLocation("2006-01-02T15:04:05", args.StartsAt, loc)
	if err != nil {
		return fmt.Errorf("error parsing start date: %v", err)
	}

	endsAt, err := time.Parse("2006-01-02", args.EndsAt)
	if err != nil {
		return fmt.Errorf("error parsing end date: %v", err)
	}

	log.Infof("Exporting %s to csv", args.InputStreamName)

	ctx := context.Background()

	eventpubsub.Init()

	if err := utils.InitEnvironmentVariables(projectsDir, args.GoEnv); err != nil {
		return fmt.Errorf("error initializing environment variables: %v", err)
	}

	settings, err := esdb.ParseConnectionString(os.Getenv("EVENTSTOREDB_URL"))
	if err != nil {
		return fmt.Errorf("error parsing connection string: %v", err)
	}

	esdbClient, err := esdb.NewClient(settings)
	if err != nil {
		return fmt.Errorf("error creating new client: %v", err)
	}

	// Fetch all data
	csvCandleInstance := eventmodels.NewCsvCandleDTO(eventmodels.StreamName(args.InputStreamName), eventmodels.CandleSavedEvent, 1)
	dataMap, err := eventservices.FetchAll(ctx, esdbClient, csvCandleInstance)
	if err != nil {
		return fmt.Errorf("error fetching all candles: %v", err)
	}

	log.Infof("Fetched %d candles\n", len(dataMap))

	// Process the data
	duration, err := helpers.GetDurationFromStreamName(args.InputStreamName)
	if err != nil {
		return fmt.Errorf("error getting duration from stream name: %v", err)
	}

	var candles eventmodels.TradingViewCandles
	for _, c := range dataMap {
		candles = append(candles, c.ToModel())
	}

	candles = utils.SortCandles(candles, duration)

	var filteredCandles eventmodels.TradingViewCandles
	for _, c := range candles {
		if !startsAt.IsZero() && c.Timestamp.Before(startsAt) {
			continue
		}

		if !endsAt.IsZero() && c.Timestamp.After(endsAt) {
			break
		}

		filteredCandles = append(filteredCandles, c)
	}

	// Write headers
	if len(filteredCandles) == 0 {
		return fmt.Errorf("no candles to export")
	}

	firstCandleStartDate := filteredCandles[0].Timestamp.Format("20060102_150405")
	lastCandleStartDate := filteredCandles[len(filteredCandles)-1].Timestamp.Format("2006-01-02")

	// Export the data
	filename := fmt.Sprintf("%s-from-%s-to-%s.csv", args.InputStreamName, firstCandleStartDate, lastCandleStartDate)
	outdir := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "data", filename)
	file, err := os.Create(outdir)
	if err != nil {
		return fmt.Errorf("error creating CSV file: %v", err)
	}

	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	candlesOUT := filteredCandles.ToCsvDTO()

	headers := helpers.GetHeadersFromStruct(*candlesOUT[0])

	writer.Write(headers)

	for _, c := range candlesOUT {
		// timeInISO, err := c.Timestamp
		// if err != nil {
		// 	return fmt.Errorf("error marshalling time to ISO: %v", err)
		// }

		// payload := []string{string(timeInISO)}
		payload := []string{}
		for _, header := range headers {
			payload = append(payload, fmt.Sprintf("%v", reflect.ValueOf(*c).FieldByName(header).Interface()))
		}

		writer.Write(payload)
	}

	if startsAt.IsZero() {
		log.Infof("Exported %d candles to %s", len(filteredCandles), outdir)
	} else {
		log.Infof("Exported %d candles to %s from %s to %s", len(filteredCandles), outdir, startsAt.Format("2006-01-02 15:04:05"), endsAt.Format("2006-01-02"))
	}

	return nil
}
