package sheets

import (
	"context"
	"fmt"
	"google.golang.org/api/sheets/v4"
	"slack-trading/src/models"
	"strconv"
	"time"
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

func fetchCandles(ctx context.Context, srv *sheets.Service) (*models.Candles, error) {
	candles := models.Candles{
		Period: 5,
	}

	fetched, err := fetchRows(ctx, srv, spreadsheetId, sheetName, "2:1010")
	if err != nil {
		return nil, err
	}

	for _, row := range fetched {
		timestampStr, ok := row[0].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[0]=%v", row[0])
		}

		timestamp, timeErr := time.Parse(time.RFC3339, timestampStr)
		if timeErr != nil {
			return nil, timeErr
		}

		lastUpdatedStr, ok := row[1].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[1]=%v", row[1])
		}

		lastUpdated, timeErr := time.Parse(time.RFC3339, lastUpdatedStr)
		if timeErr != nil {
			return nil, timeErr
		}

		openPriceStr, ok := row[3].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[3]=%v", row[3])
		}

		openPrice, parseErr := strconv.ParseFloat(openPriceStr, 64)
		if parseErr != nil {
			return nil, parseErr
		}

		highPriceStr, ok := row[4].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[4]=%v", row[4])
		}

		highPrice, parseErr := strconv.ParseFloat(highPriceStr, 64)
		if parseErr != nil {
			return nil, parseErr
		}

		lowPriceStr, ok := row[5].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[5]=%v", row[5])
		}

		lowPrice, parseErr := strconv.ParseFloat(lowPriceStr, 64)
		if parseErr != nil {
			return nil, parseErr
		}

		closePriceStr, ok := row[6].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[6]=%v", row[6])
		}

		closePrice, parseErr := strconv.ParseFloat(closePriceStr, 64)
		if parseErr != nil {
			return nil, parseErr
		}

		candles.Data = append(candles.Data, models.Candle{
			Timestamp:   timestamp,
			LastUpdated: lastUpdated,
			Open:        openPrice,
			High:        highPrice,
			Low:         lowPrice,
			Close:       closePrice,
		})
	}

	return &candles, nil
}

func FetchCandles(ctx context.Context) (*models.Candles, error) {
	mu.Lock()
	defer mu.Unlock()

	return fetchCandles(ctx, service)
}
