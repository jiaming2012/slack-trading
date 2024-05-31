package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
	"slack-trading/src/eventservices"
	"slack-trading/src/utils"
)

func getHeaders(dataMap []map[string]interface{}) []string {
	var header []string
	if len(dataMap) == 0 {
		return header
	}

	for key := range dataMap[0] {
		if key == "Timestamp" {
			continue
		}

		header = append(header, key)
	}

	return header
}

func getHeadersFromStruct(i interface{}) []string {
	t := reflect.TypeOf(i)
	headers := make([]string, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		headers[i] = t.Field(i).Name
	}
	return headers
}

func getDurationFromStreamName(streamName string) (time.Duration, error) {
	if streamName[0:7] != "candles" {
		return 0, fmt.Errorf("invalid stream name: expected stream name to start with 'candles'")
	}

	components := strings.Split(streamName, "-")
	if len(components) != 3 {
		return 0, fmt.Errorf("invalid stream name: expected stream name to have 3 components ['candles', 'underlying_symbol', 'duration], found %v components", components)
	}

	duration := components[2]

	// check if duration has D or W
	if strings.Contains(duration, "D") {
		// check number of days
		daysStr := strings.Split(duration, "D")
		if len(daysStr) != 2 {
			return 0, fmt.Errorf("invalid duration: expected duration to have 2 components ['number', 'D'], found %v components", daysStr)
		}

		days, err := strconv.Atoi(daysStr[0])
		if err != nil {
			return 0, fmt.Errorf("invalid duration: expected duration to represent number of days, found %v", daysStr[0])
		}

		return time.Duration(days) * 24 * time.Hour, nil
	}

	if strings.Contains(duration, "W") {
		// check number of weeks
		weeksStr := strings.Split(duration, "W")
		if len(weeksStr) != 2 {
			return 0, fmt.Errorf("invalid duration: expected duration to have 2 components ['number', 'W'], found %v components", weeksStr)
		}

		weeks, err := strconv.Atoi(weeksStr[0])
		if err != nil {
			return 0, fmt.Errorf("invalid duration: expected duration to represent number of weeks, found %v", weeksStr[0])
		}

		return time.Duration(weeks) * 7 * 24 * time.Hour, nil
	}

	hours, err := strconv.Atoi(duration)
	if err != nil {
		return 0, fmt.Errorf("invalid duration: expected duration to be represent number of hours, found %v", duration)
	}

	return time.Duration(hours) * time.Hour, nil
}

func main() {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		panic("missing PROJECTS_DIR environment variable")
	}

	if len(os.Args) < 2 {
		panic("missing input stream")
	}

	inputStream := os.Args[1]
	if inputStream == "" {
		panic("missing input stream")
	}

	var startDate time.Time
	var endDate time.Time
	if len(os.Args) > 3 {
		startDateStr := os.Args[2]
		s, err := time.Parse("2006-01-02", startDateStr)
		if err != nil {
			log.Fatalf("error parsing start date: %v", err)
		}

		endDateStr := os.Args[3]
		e, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			log.Fatalf("error parsing end date: %v", err)
		}

		startDate = s
		endDate = e
	}

	log.Infof("Exporting %s to csv", inputStream)

	ctx := context.Background()

	eventpubsub.Init()

	if err := utils.InitEnvironmentVariables(); err != nil {
		log.Fatalf("error initializing environment variables: %v", err)
	}

	settings, err := esdb.ParseConnectionString(os.Getenv("EVENTSTOREDB_URL"))
	if err != nil {
		log.Fatalf("error parsing connection string: %v", err)
	}

	esdbClient, err := esdb.NewClient(settings)
	if err != nil {
		log.Fatalf("error creating new client: %v", err)
	}

	// Fetch all data
	csvCandleInstance := eventmodels.NewCsvCandleDTO(eventmodels.StreamName(inputStream), eventmodels.CandleSavedEvent, 1)
	dataMap, err := eventservices.FetchAll(ctx, esdbClient, csvCandleInstance)
	if err != nil {
		log.Fatalf("error fetching all candles: %v", err)
	}

	log.Infof("Fetched %d candles\n", len(dataMap))

	// Process the data
	duration, err := getDurationFromStreamName(inputStream)
	if err != nil {
		log.Fatalf("error getting duration from stream name: %v", err)
	}

	var candles eventmodels.TradingViewCandles
	for _, c := range dataMap {
		candles = append(candles, c.ToModel())
	}

	candles = utils.SortCandles(candles, duration)

	var filteredCandles eventmodels.TradingViewCandles
	for _, c := range candles {
		if !startDate.IsZero() && c.Timestamp.Before(startDate) {
			continue
		}

		if !endDate.IsZero() && c.Timestamp.After(endDate) {
			break
		}

		filteredCandles = append(filteredCandles, c)
	}

	// Write headers
	if len(filteredCandles) == 0 {
		log.Fatalf("no candles to export")
	}

	firstCandleStartDate := filteredCandles[0].Timestamp.Format("2006-01-02")
	lastCandleStartDate := filteredCandles[len(filteredCandles)-1].Timestamp.Format("2006-01-02")

	// Export the data
	outdir := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "data", fmt.Sprintf("%s-from-%s-to-%s.csv", inputStream, firstCandleStartDate, lastCandleStartDate))
	file, err := os.Create(outdir)
	if err != nil {
		log.Fatalf("error creating CSV file: %v", err)
	}

	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	candlesOUT := filteredCandles.ToCsvDTO()

	headers := getHeadersFromStruct(*candlesOUT[0])

	writer.Write(headers)

	for _, c := range candlesOUT {
		// timeInISO, err := c.Timestamp
		// if err != nil {
		// 	log.Fatalf("error marshalling time to ISO: %v", err)
		// }

		// payload := []string{string(timeInISO)}
		payload := []string{}
		for _, header := range headers {
			payload = append(payload, fmt.Sprintf("%v", reflect.ValueOf(*c).FieldByName(header).Interface()))
		}

		writer.Write(payload)
	}

	if startDate.IsZero() {
		log.Infof("Exported %d candles to %s", len(filteredCandles), outdir)
	} else {
		log.Infof("Exported %d candles to %s from %s to %s", len(filteredCandles), outdir, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	}
}
