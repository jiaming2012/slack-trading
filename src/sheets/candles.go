package sheets

import (
	"context"
	"google.golang.org/api/sheets/v4"
	"slack-trading/src/models"
)

const btcUsdSheetName = "BTCUSD"

func appendCandle(ctx context.Context, srv *sheets.Service, candle *models.Candle) error {
	candles := make(models.Candles, 0)
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
