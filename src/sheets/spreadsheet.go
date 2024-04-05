package sheets

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/sheets/v4"

	"slack-trading/src/models"
)

func CreateSpreadsheet(ctx context.Context, srv *sheets.Service, title string) (*sheets.Spreadsheet, error) {
	// Create the new spreadsheet
	spreadsheet := &sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: title,
		},
	}

	return srv.Spreadsheets.Create(spreadsheet).Context(ctx).Do()
}

func MoveSpreadsheet(ctx context.Context, sheetSrv *sheets.Spreadsheet, driveSrv *drive.Service, folderId string) error {
	file, err := driveSrv.Files.Get(sheetSrv.SpreadsheetId).Do()
	if err != nil {
		log.Fatalf("Failed to get file: %v", err)
	}

	var call *drive.FilesUpdateCall
	call = driveSrv.Files.Update(file.Id, &drive.File{}).AddParents(folderId)

	if len(file.Parents) > 0 {
		call = call.RemoveParents(file.Parents[0])
	}

	if _, err := call.Do(); err != nil {
		log.Fatalf("Failed to move file: %v", err)
	}

	return err
}

func AppendRows(ctx context.Context, srv *sheets.Service, spreadsheetId string, sheetName string, values [][]interface{}) error {
	row := &sheets.ValueRange{
		Values: values,
	}

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

func fetchLastXRows(ctx context.Context, srv *sheets.Service, spreadsheetId string, sheetName string, numRows int64) (models.Rows, error) {
	rangeString := fmt.Sprintf("%s!A:Z", sheetName)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, rangeString).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve data from sheet: %v", err)
	}

	totalRows := int64(len(resp.Values))
	startRow := totalRows - numRows
	if startRow < 0 {
		startRow = 0
	}

	rangeString = fmt.Sprintf("%s!A%d:Z", sheetName, startRow+1)
	resp, err = srv.Spreadsheets.Values.Get(spreadsheetId, rangeString).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve data from sheet: %v", err)
	}

	return resp.Values, nil
}

func fetchRows(ctx context.Context, srv *sheets.Service, spreadsheetId string, sheetName string, cells string) (models.Rows, error) {
	sheetRange := fmt.Sprintf("%s!%s", sheetName, cells)
	response, err := srv.Spreadsheets.Values.Get(spreadsheetId, sheetRange).Context(ctx).Do()
	if err != nil || response.HTTPStatusCode != 200 {
		return nil, err
	}

	return response.Values, nil
}
