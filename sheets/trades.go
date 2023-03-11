package sheets

import (
	"context"
	"fmt"
	"google.golang.org/api/sheets/v4"
	models2 "slack-trading/src/models"
	"strconv"
	"time"
)

// trades Google sheet
// https://docs.google.com/spreadsheets/d/1_bTrqV8RTAQY6j33f_G0myA2-rF-s60FI4HwLbyvYo0/edit#gid=0

const spreadsheetId = "1_bTrqV8RTAQY6j33f_G0myA2-rF-s60FI4HwLbyvYo0"
const sheetName = "Trades"

func FetchTrades(ctx context.Context, srv *sheets.Service, symbol string) (models2.Trades, error) {
	trades := make(models2.Trades, 0)

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

		_symbol, ok := row[1].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[1]=%v", row[1])
		}

		volumeString, ok := row[2].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[2]=%v", row[2])
		}

		volume, parseErr := strconv.ParseFloat(volumeString, 64)
		if parseErr != nil {
			return nil, parseErr
		}

		priceString, ok := row[3].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[3]=%v", row[3])
		}

		price, parseErr := strconv.ParseFloat(priceString, 64)
		if parseErr != nil {
			return nil, parseErr
		}

		trades = append(trades, models2.Trade{
			Time:   timestamp,
			Symbol: _symbol,
			Volume: volume,
			Price:  price,
		})
	}

	return trades, nil
}

func AppendTrade(ctx context.Context, srv *sheets.Service, tr *models2.Trade) error {
	trades := make(models2.Trades, 0)
	trades.Add(tr)
	values := trades.ToRows()
	return appendRows(ctx, srv, spreadsheetId, sheetName, values)
}
