package run

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"time"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/cmd/stats/export_data/helpers"
	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
	"slack-trading/src/eventservices"
	"slack-trading/src/utils"
)

type RunArgs struct {
	InputStreamName string
	StartsAt        time.Time
	EndsAt          time.Time
	GoEnv           string
}

type RunOutput struct {
	ExportedFilepath string
}

func Run(args RunArgs) (RunOutput, error) {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		panic("missing PROJECTS_DIR environment variable")
	}

	ctx := context.Background()

	filename := fmt.Sprintf("%s-from-%s-to-%s.csv", args.InputStreamName, args.StartsAt.Format("20060102_150405"), args.EndsAt.Format("20060102_150405"))
	outdir := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "data", filename)

	// check if file exists
	if _, err := os.Stat(outdir); err == nil {
		log.Infof("Data file %s already exists", outdir)
		return RunOutput{
			ExportedFilepath: outdir,
		}, nil
	}

	log.Infof("Exporting %s to csv", args.InputStreamName)

	eventpubsub.Init()

	if err := utils.InitEnvironmentVariables(projectsDir, args.GoEnv); err != nil {
		return RunOutput{}, fmt.Errorf("error initializing environment variables: %v", err)
	}

	settings, err := esdb.ParseConnectionString(os.Getenv("EVENTSTOREDB_URL"))
	if err != nil {
		return RunOutput{}, fmt.Errorf("error parsing connection string: %v", err)
	}

	esdbClient, err := esdb.NewClient(settings)
	if err != nil {
		return RunOutput{}, fmt.Errorf("error creating new client: %v", err)
	}

	// Fetch all data
	csvCandleInstance := eventmodels.NewCsvCandleDTO(eventmodels.StreamName(args.InputStreamName), eventmodels.CandleSavedEvent, 1)
	dataMap, err := eventservices.FetchAll(ctx, esdbClient, csvCandleInstance)
	if err != nil {
		return RunOutput{}, fmt.Errorf("error fetching all candles: %v", err)
	}

	log.Infof("Fetched %d candles", len(dataMap))

	// Process the data
	duration, err := helpers.GetDurationFromStreamName(args.InputStreamName)
	if err != nil {
		return RunOutput{}, fmt.Errorf("error getting duration from stream name: %v", err)
	}

	var candles eventmodels.TradingViewCandles
	for _, candleDTO := range dataMap {
		c, err := candleDTO.ToModel()
		if err != nil {
			return RunOutput{}, fmt.Errorf("error converting to model: %v", err)
		}

		candles = append(candles, c)
	}

	candles = utils.SortCandles(candles, duration)

	var filteredCandles eventmodels.TradingViewCandles
	for _, c := range candles {
		if c.Timestamp.Before(args.StartsAt) {
			continue
		}

		if c.Timestamp.After(args.EndsAt) {
			break
		}

		filteredCandles = append(filteredCandles, c)
	}

	// Checks
	if len(filteredCandles) == 0 {
		return RunOutput{}, fmt.Errorf("no candles to export")
	}

	firstCandleTimestamp := filteredCandles[0].Timestamp
	lastCandleTimestamp := filteredCandles[len(filteredCandles)-1].Timestamp

	if firstCandleTimestamp.After(args.StartsAt) {
		return RunOutput{}, fmt.Errorf("start candle date %v is after start: %v", firstCandleTimestamp, args.StartsAt)
	}

	if lastCandleTimestamp.Add(duration).Before(args.EndsAt) {
		return RunOutput{}, fmt.Errorf("end candle date %v is before end: %v", lastCandleTimestamp.Add(duration), args.EndsAt)
	}

	// Export the data
	dir := filepath.Dir(outdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}

	file, err := os.Create(outdir)
	if err != nil {
		return RunOutput{}, fmt.Errorf("error creating CSV file: %v", err)
	}

	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	candlesOUT := filteredCandles.ToCsvDTO()

	headers := helpers.GetHeadersFromStruct(*candlesOUT[0])

	writer.Write(headers)

	for _, c := range candlesOUT {
		payload := []string{}
		for _, header := range headers {
			payload = append(payload, fmt.Sprintf("%v", reflect.ValueOf(*c).FieldByName(header).Interface()))
		}

		writer.Write(payload)
	}

	if args.StartsAt.IsZero() {
		log.Infof("Exported %d candles to %s", len(filteredCandles), outdir)
	} else {
		log.Infof("Exported %d candles to %s from %s to %s", len(filteredCandles), outdir, args.StartsAt.Format("2006-01-02 15:04:05"), args.EndsAt.Format("2006-01-02"))
	}

	return RunOutput{
		ExportedFilepath: outdir,
	}, nil
}
