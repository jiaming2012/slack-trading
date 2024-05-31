package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path"

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

	csvCandleInstance := eventmodels.NewCsvCandleDTO(eventmodels.StreamName(inputStream), eventmodels.CandleSavedEvent, 1)
	dataMap, err := eventservices.FetchAllData(ctx, esdbClient, csvCandleInstance)
	if err != nil {
		log.Fatalf("error fetching all candles: %v", err)
	}

	log.Infof("Fetched %d candles\n", len(dataMap))

	// Export the data
	outdir := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "data", fmt.Sprintf("%s.csv", inputStream))
	file, err := os.Create(outdir)
	if err != nil {
		log.Fatalf("error creating CSV file: %v", err)
	}

	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write headers
	headers := []string{"Timestamp"}
	headers = append(headers, getHeaders(dataMap)...)

	writer.Write(headers)

	for _, c := range dataMap {
		timestampStr := c["time"].(string)
		// timeInISO, err := time.Parse(time.RFC3339, timestampStr)
		// if err != nil {
		// 	log.Fatalf("error parsing time: %v", err)
		// }

		payload := []string{timestampStr}
		for _, header := range headers {
			if header == "Timestamp" {
				continue
			}

			payload = append(payload, fmt.Sprintf("%v", c[header]))
		}

		writer.Write(payload)
		// writer.Write([]string{timeInISO, fmt.Sprintf("%f", c.Open), fmt.Sprintf("%f", c.High), fmt.Sprintf("%f", c.Low), fmt.Sprintf("%f", c.Close), fmt.Sprintf("%f", c.UpTrend), fmt.Sprintf("%f", c.DownTrend)})
	}

	log.Infof("Exported %d candles to %s\n", len(dataMap), outdir)
}
