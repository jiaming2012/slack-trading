package main

import (
	"context"
	"encoding/base64"
	"fmt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"os"
)

type Data struct {
	Id    string
	Name  string
	Email string
}

func setup(ctx context.Context) (*sheets.Service, error) {
	// get bytes from base64 encoded google service accounts key
	credBytes, err := base64.StdEncoding.DecodeString(os.Getenv("KEY_JSON_BASE64"))
	if err != nil {
		return nil, err
	}

	// authenticate and get configuration
	config, err := google.JWTConfigFromJSON(credBytes, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		return nil, err
	}

	// create client with config and context
	client := config.Client(ctx)

	// create new service using client
	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return srv, nil
}

func appendRow(ctx context.Context, srv *sheets.Service, spreadsheetId string, sheetName string) {
	row := &sheets.ValueRange{
		Values: [][]interface{}{{"9", "ABC", "abc@gmail.com"}},
	}

	response2, err := srv.Spreadsheets.Values.Append(spreadsheetId, sheetName, row).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Context(ctx).Do()
	if err != nil || response2.HTTPStatusCode != 200 {
		log.Error(err)
		return
	}
}

func updateRow(ctx context.Context, srv *sheets.Service, spreadsheetId string, sheetName string) {
	// The A1 notation of cells range to update.
	range2 := fmt.Sprintf("%s!A1:C1", sheetName)

	// prepare data for update cells
	row := &sheets.ValueRange{
		Values: [][]interface{}{{"3", "XYZ", "xyz@gmail.com"}},
	}

	// update cells in given range
	_, err := srv.Spreadsheets.Values.Update(spreadsheetId, range2, row).ValueInputOption("USER_ENTERED").Context(ctx).Do()
	if err != nil {
		log.Fatal(err)
	}
}

func fetchRows(ctx context.Context, srv *sheets.Service, spreadsheetId string, sheetName string, cells string) ([]Data, error) {
	sheetRange := fmt.Sprintf("%s!%s", sheetName, cells)

	response1, err := srv.Spreadsheets.Values.Get(spreadsheetId, sheetRange).Context(ctx).Do()
	if err != nil || response1.HTTPStatusCode != 200 {
		return nil, err
	}

	rows := make([]Data, 0)
	for _, row := range response1.Values {
		id, ok := row[0].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[0]=%v", row[0])
		}

		name, ok := row[1].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[1]=%v", row[1])
		}

		email, ok := row[2].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[2]=%v", row[2])
		}

		rows = append(rows, Data{
			Id:    id,
			Name:  name,
			Email: email,
		})
	}

	return rows, nil
}

func main() {
	// https://docs.google.com/spreadsheets/d/1_bTrqV8RTAQY6j33f_G0myA2-rF-s60FI4HwLbyvYo0/edit#gid=0
	spreadsheetId := "1_bTrqV8RTAQY6j33f_G0myA2-rF-s60FI4HwLbyvYo0"

	// create api context
	ctx := context.Background()

	// authenticate and setup service
	srv, err := setup(ctx)
	if err != nil {
		log.Fatal(err)
	}

	//appendRow(ctx, srv, spreadsheetId, "Sheet1")
	//updateRow(ctx, srv, spreadsheetId, "Sheet2")
	rows, err := fetchRows(ctx, srv, spreadsheetId, "Sheet1", "A3:C7")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(rows)
}
