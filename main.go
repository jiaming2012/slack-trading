package main

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"slack-trading/models"
	"slack-trading/sheets"
	"time"
)

func main() {
	// create api context
	ctx := context.Background()

	// authenticate and setup service
	srv, err := sheets.Setup(ctx)
	if err != nil {
		log.Fatalf("setup failed: %v", err)
	}

	trade := &models.Trade{
		Time:   time.Now(),
		Symbol: "ETHUSD",
		Volume: -1.2,
		Price:  2340.60,
	}

	err = sheets.AppendTrade(ctx, srv, trade)
	if err != nil {
		log.Fatal(err)
	}
	//appendRow(ctx, srv, spreadsheetId, "Sheet1")
	//updateRow(ctx, srv, spreadsheetId, "Sheet2")
	//rows, err := fetchRows(ctx, srv, spreadsheetId, "Sheet1", "A3:C7")
	//if err != nil {
	//	log.Fatal(err)
	//}

	trades, err := sheets.FetchTrades(ctx, srv, "ETHUSD")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(trades)
}
