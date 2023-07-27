package sheets

import (
	"context"
	"google.golang.org/api/sheets/v4"
	"slack-trading/src/models"
)

const btcUsdSheetName = "BTCUSD"

func appendCandle(ctx context.Context, srv *sheets.Service, candle *models.Candle) error {
	candles := models.Candles{}
	candles.Add(candle)
	values := candles.ToRows()
	return appendRows(ctx, srv, spreadsheetId, btcUsdSheetName, values)
}

func AppendCandle(ctx context.Context, candle *models.Candle) error {
	mu.Lock()
	defer mu.Unlock()

	var candles models.Candles
	candles.Add(candle)
	return appendCandle(ctx, service, candle)
}

func fetchLastXCandles(ctx context.Context, srv *sheets.Service, numRows int64) (*models.Candles, error) {
	rows, err := fetchLastXRows(ctx, srv, spreadsheetId, btcUsdSheetName, numRows)
	if err != nil {
		return nil, err
	}

	return rows.ConvertToCandles()
}

func fetchCandles(ctx context.Context, srv *sheets.Service) (*models.Candles, error) {
	rows, err := fetchRows(ctx, srv, spreadsheetId, btcUsdSheetName, "2:1010")
	if err != nil {
		return nil, err
	}

	return rows.ConvertToCandles()
}

func FetchLastXCandles(ctx context.Context, numRows int64) (*models.Candles, error) {
	mu.Lock()
	defer mu.Unlock()

	return fetchLastXCandles(ctx, service, numRows)
}

func FetchCandles(ctx context.Context) (*models.Candles, error) {
	mu.Lock()
	defer mu.Unlock()

	return fetchCandles(ctx, service)
}
