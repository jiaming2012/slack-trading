package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
	"slack-trading/src/sheets"
)

func findFirstEventNumber(db *esdb.Client, streamName eventmodels.StreamName) uint64 {
	stream, err := db.ReadStream(context.Background(), string(streamName), esdb.ReadStreamOptions{
		Direction: esdb.Forwards,
		From:      esdb.Start{},
	}, 1)

	if err != nil {
		panic(err)
	}

	event, err := stream.Recv()
	if err != nil {
		panic(err)
	}

	return event.Event.EventNumber
}

func loadOptionChainTicks(db *esdb.Client, streamName eventmodels.StreamName, contract1 eventmodels.OptionContractV1, output1 *[]eventmodels.OptionChainTickV1, contract2 eventmodels.OptionContractV1, output2 *[]eventmodels.OptionChainTickV1) {
	var pos esdb.StreamPosition = esdb.End{}
	var eventNumber uint64

	firstEventNumber := findFirstEventNumber(db, eventmodels.OptionChainTickStream)

	for {
		stream, err := db.ReadStream(context.Background(), string(streamName), esdb.ReadStreamOptions{
			Direction: esdb.Backwards,
			From:      pos,
		}, 100)

		if err != nil {
			panic(err)
		}

		for {
			event, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				panic(err)
			}

			var optionChainTick eventmodels.OptionChainTickV1
			if err := json.Unmarshal(event.Event.Data, &optionChainTick); err != nil {
				panic(err)
			}

			if optionChainTick.OptionContractID == contract1.Meta.GetEventStreamID() {
				*output1 = append(*output1, optionChainTick)
			}

			if optionChainTick.OptionContractID == contract2.Meta.GetEventStreamID() {
				*output2 = append(*output2, optionChainTick)
			}

			eventNumber = event.Event.EventNumber

			if math.Mod(float64(eventNumber), 10000) == 0 {
				log.Infof("Loading at position: %v\n", eventNumber)
			}
		}

		pos = esdb.Revision(eventNumber)

		if eventNumber == firstEventNumber {
			break
		}
	}
}

func loadStockTicks(db *esdb.Client, streamName eventmodels.StreamName, output *[]eventmodels.StockTickV1) {
	var pos esdb.StreamPosition = esdb.End{}
	var eventNumber uint64

	for {
		stream, err := db.ReadStream(context.Background(), string(streamName), esdb.ReadStreamOptions{
			Direction: esdb.Backwards,
			From:      pos,
		}, 100)

		if err != nil {
			log.Fatalf("Failed to read stream: %v", err)
		}

		for {
			event, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				log.Fatalf("Failed to receive event: %v", err)
			}

			var stockTick eventmodels.StockTickV1
			if err := json.Unmarshal(event.Event.Data, &stockTick); err != nil {
				log.Fatalf("Failed to unmarshal event data: %v", err)
			}

			*output = append(*output, stockTick)

			eventNumber = event.Event.EventNumber
		}

		pos = esdb.Revision(eventNumber)

		if eventNumber == 0 {
			break
		}
	}
}

type NormalizedData struct {
	Timestamp    time.Time
	StockPrice   float64
	OptionPrice1 float64
	OptionPrice2 float64
}

type NormalizedDataSlice []NormalizedData

func NormalizeTicks(stockTicks []eventmodels.StockTickV1, optionChainTicks1 []eventmodels.OptionChainTickV1, optionChainTicks2 []eventmodels.OptionChainTickV1) []NormalizedData {
	option1TickMap := make(map[time.Time]eventmodels.OptionChainTickV1)
	for _, optionTick := range optionChainTicks1 {
		option1TickMap[optionTick.Timestamp] = optionTick
	}

	option2TickMap := make(map[time.Time]eventmodels.OptionChainTickV1)
	for _, optionTick := range optionChainTicks2 {
		option2TickMap[optionTick.Timestamp] = optionTick
	}

	normalizedTicks := []NormalizedData{}
	for _, stockTick := range stockTicks {
		option1Tick, found := option1TickMap[stockTick.Timestamp]
		if !found {
			continue
		}

		option2Tick, found := option2TickMap[stockTick.Timestamp]
		if !found {
			continue
		}

		normalizedTicks = append(normalizedTicks, NormalizedData{
			Timestamp:    stockTick.Timestamp,
			StockPrice:   stockTick.LastPrice,
			OptionPrice1: option1Tick.Last,
			OptionPrice2: option2Tick.Last,
		})
	}

	return normalizedTicks
}

func (data NormalizedDataSlice) ToRows() [][]interface{} {
	results := make([][]interface{}, 0)

	for i := len(data) - 1; i >= 0; i-- {
		results = append(results, []interface{}{
			data[i].Timestamp.Format(time.RFC3339),
			data[i].StockPrice,
			data[i].OptionPrice1,
			data[i].OptionPrice2,
		})
	}

	return results
}

func main() {
	ctx := context.Background()
	tickDataFolderID := "1xd5LrceF7r8TymrmwR1daO7gSxT-PLhc"

	// Set up
	eventmodels.InitializeGlobalDispatcher()
	eventpubsub.Init()

	// Create EventStore client
	eventStoreDbURL := os.Getenv("EVENTSTOREDB_URL")

	settings, err := esdb.ParseConnectionString(eventStoreDbURL)
	if err != nil {
		log.Fatalf("Failed to parse connection string: %v", err)
	}

	db, err := esdb.NewClient(settings)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create Google Sheets API client
	srv, drive, err := sheets.Init(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize google sheets: %v", err)
	}

	spreadsheet, err := sheets.CreateSpreadsheet(ctx, srv, fmt.Sprintf("Stock Ticks - %s", time.Now().Format("01/02/2006 15:04")))
	if err != nil {
		log.Fatalf("Failed to create spreadsheet: %v", err)
	}

	fmt.Println("Spreadsheet created: ", spreadsheet.SpreadsheetUrl)

	if err := sheets.MoveSpreadsheet(ctx, spreadsheet, drive, tickDataFolderID); err != nil {
		log.Fatalf("Failed to move spreadsheet: %v", err)
	}

	stockTicks := []eventmodels.StockTickV1{}

	loadStockTicks(db, eventmodels.StockTickStream, &stockTicks)

	fmt.Printf("Loaded %d stock ticks\n", len(stockTicks))

	optionContract1 := eventmodels.OptionContractV1{}

	optionContract2 := eventmodels.OptionContractV1{}

	panic("implement fetch option contracts")

	// optionContract1 := eventmodels.CoinOptionContracts[11]

	// optionContract2 := eventmodels.CoinOptionContracts[15]

	fmt.Printf("Loading data for option: %s\n", optionContract1.Description)

	fmt.Printf("Loading data for option: %s\n", optionContract2.Description)

	optionChainTicks1 := []eventmodels.OptionChainTickV1{}

	optionChainTicks2 := []eventmodels.OptionChainTickV1{}

	loadOptionChainTicks(db, eventmodels.OptionChainTickStream, optionContract1, &optionChainTicks1, optionContract2, &optionChainTicks2)

	fmt.Printf("Loaded %d option chain ticks\n", len(optionChainTicks1))

	normalizedTicks := NormalizeTicks(stockTicks, optionChainTicks1, optionChainTicks2)

	fmt.Printf("Normalized %d ticks\n", len(normalizedTicks))

	// create header row
	headerRow := [][]interface{}{
		{"Timestamp", "Stock Price", fmt.Sprintf(optionContract1.Description), fmt.Sprintf(optionContract2.Description)},
	}

	if sheets.AppendRows(ctx, srv, spreadsheet.SpreadsheetId, "Sheet1", headerRow) != nil {
		log.Fatalf("Failed to save header row: %v", err)
	}

	values := NormalizedDataSlice(normalizedTicks).ToRows()

	if sheets.AppendRows(ctx, srv, spreadsheet.SpreadsheetId, "Sheet1", values) != nil {
		log.Fatalf("Failed to save rows: %v", err)
	}
}
