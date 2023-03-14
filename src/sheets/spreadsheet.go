package sheets

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/sheets/v4"
	"slack-trading/src/models"
)

func appendRows(ctx context.Context, srv *sheets.Service, spreadsheetId string, sheetName string, values [][]interface{}) error {
	row := &sheets.ValueRange{
		Values: values,
	}

	fmt.Println("apR: ", values)

	response, err := srv.Spreadsheets.Values.Append(spreadsheetId, sheetName, row).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Context(ctx).Do()
	if err != nil {
		return err
	}

	if response.HTTPStatusCode != 200 {
		return fmt.Errorf("invalid http status code: %v", response.HTTPStatusCode)
	}

	return nil
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

func fetchRows(ctx context.Context, srv *sheets.Service, spreadsheetId string, sheetName string, cells string) (models.Rows, error) {
	sheetRange := fmt.Sprintf("%s!%s", sheetName, cells)

	response, err := srv.Spreadsheets.Values.Get(spreadsheetId, sheetRange).Context(ctx).Do()
	if err != nil || response.HTTPStatusCode != 200 {
		return nil, err
	}

	return response.Values, nil
}
