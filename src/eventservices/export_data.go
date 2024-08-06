package eventservices

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	"github.com/gocarina/gocsv"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/cmd/stats/export_data/helpers"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventpubsub"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func ExportData(args eventmodels.ExportDataRunArgs) (eventmodels.ExportDataRunOutput, error) {
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
		return eventmodels.ExportDataRunOutput{
			ExportedFilepath: outdir,
		}, nil
	}

	log.Infof("Exporting %s to csv", args.InputStreamName)

	eventpubsub.Init()

	if err := utils.InitEnvironmentVariables(projectsDir, args.GoEnv); err != nil {
		return eventmodels.ExportDataRunOutput{}, fmt.Errorf("error initializing environment variables: %v", err)
	}

	settings, err := esdb.ParseConnectionString(os.Getenv("EVENTSTOREDB_URL"))
	if err != nil {
		return eventmodels.ExportDataRunOutput{}, fmt.Errorf("error parsing connection string: %v", err)
	}

	esdbClient, err := esdb.NewClient(settings)
	if err != nil {
		return eventmodels.ExportDataRunOutput{}, fmt.Errorf("error creating new client: %v", err)
	}

	// Fetch all data
	csvCandleInstance := eventmodels.NewCsvCandleDTO(eventmodels.StreamName(args.InputStreamName), eventmodels.CandleSavedEvent, 1)
	dataMap, err := FetchAll(ctx, esdbClient, csvCandleInstance)
	if err != nil {
		return eventmodels.ExportDataRunOutput{}, fmt.Errorf("error fetching all candles: %v", err)
	}

	log.Infof("Fetched %d candles", len(dataMap))

	// Process the data
	duration, err := helpers.GetDurationFromStreamName(args.InputStreamName)
	if err != nil {
		return eventmodels.ExportDataRunOutput{}, fmt.Errorf("error getting duration from stream name: %v", err)
	}

	var candles eventmodels.TradingViewCandles
	for _, candleDTO := range dataMap {
		c, err := candleDTO.ToModel()
		if err != nil {
			return eventmodels.ExportDataRunOutput{}, fmt.Errorf("error converting to model: %v", err)
		}

		candles = append(candles, c)
	}

	candles = eventmodels.SortCandles(candles, duration)

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
		return eventmodels.ExportDataRunOutput{}, fmt.Errorf("no candles to export")
	}

	firstCandleTimestamp := filteredCandles[0].Timestamp
	lastCandleTimestamp := filteredCandles[len(filteredCandles)-1].Timestamp

	if firstCandleTimestamp.After(args.StartsAt) {
		return eventmodels.ExportDataRunOutput{}, fmt.Errorf("start candle date %v is after start: %v", firstCandleTimestamp, args.StartsAt)
	}

	if lastCandleTimestamp.Add(duration).Before(args.EndsAt) {
		return eventmodels.ExportDataRunOutput{}, fmt.Errorf("end candle date %v is before end: %v", lastCandleTimestamp.Add(duration), args.EndsAt)
	}

	// Export the data
	dir := filepath.Dir(outdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}

	file, err := os.Create(outdir)
	if err != nil {
		return eventmodels.ExportDataRunOutput{}, fmt.Errorf("error creating CSV file: %v", err)
	}

	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	candlesOUT := filteredCandles.ToDTO()

	if err := gocsv.MarshalFile(&candlesOUT, file); err != nil {
		return eventmodels.ExportDataRunOutput{}, fmt.Errorf("error marshalling file: %v", err)
	}

	if args.StartsAt.IsZero() {
		log.Infof("Exported %d candles to %s", len(filteredCandles), outdir)
	} else {
		log.Infof("Exported %d candles to %s from %s to %s", len(filteredCandles), outdir, args.StartsAt.Format("2006-01-02 15:04:05"), args.EndsAt.Format("2006-01-02"))
	}

	return eventmodels.ExportDataRunOutput{
		ExportedFilepath: outdir,
	}, nil
}
