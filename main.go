package main

import (
	"context"
	"encoding/base64"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"os"
)

func appendRow(ctx context.Context, spreadsheetId string, sheetId int) {
	// get bytes from base64 encoded google service accounts key
	credBytes, err := base64.StdEncoding.DecodeString(os.Getenv("KEY_JSON_BASE64"))
	if err != nil {
		log.Error(err)
		return
	}

	// authenticate and get configuration
	config, err := google.JWTConfigFromJSON(credBytes, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Error(err)
		return
	}

	// create client with config and context
	client := config.Client(ctx)

	// create new service using client
	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Error(err)
		return
	}

	response1, err := srv.Spreadsheets.Get(spreadsheetId).Fields("sheets(properties(sheetId,title))").Do()
	if err != nil || response1.HTTPStatusCode != 200 {
		log.Error(err)
		return
	}

	sheetName := ""
	for _, v := range response1.Sheets {
		prop := v.Properties
		if prop.SheetId == int64(sheetId) {
			sheetName = prop.Title
			break
		}
	}

	if len(sheetName) == 0 {
		log.Error("failed to find sheetId %v", sheetId)
		return
	}

	// Append value to the sheet.
	row := &sheets.ValueRange{
		Values: [][]interface{}{{"1", "ABC", "abc@gmail.com"}},
	}

	response2, err := srv.Spreadsheets.Values.Append(spreadsheetId, sheetName, row).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Context(ctx).Do()
	if err != nil || response2.HTTPStatusCode != 200 {
		log.Error(err)
		return
	}
}

func main() {
	// https://docs.google.com/spreadsheets/d/1_bTrqV8RTAQY6j33f_G0myA2-rF-s60FI4HwLbyvYo0/edit#gid=0
	spreadsheetId := "1_bTrqV8RTAQY6j33f_G0myA2-rF-s60FI4HwLbyvYo0"
	sheetId := 0

	// create api context
	ctx := context.Background()

	appendRow(ctx, spreadsheetId, sheetId)
}
