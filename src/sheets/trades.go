package sheets

import (
	"context"
	"fmt"
	"google.golang.org/api/sheets/v4"
	"slack-trading/src/models"
	"strconv"
	"sync"
	"time"
)

// trades Google sheet
// https://docs.google.com/spreadsheets/d/1_bTrqV8RTAQY6j33f_G0myA2-rF-s60FI4HwLbyvYo0/edit#gid=0

const spreadsheetId = "1_bTrqV8RTAQY6j33f_G0myA2-rF-s60FI4HwLbyvYo0"
const sheetName = "Trades"

var mu sync.Mutex

func fetchTrades(ctx context.Context, srv *sheets.Service) (models.Trades, error) {
	trades := make(models.Trades, 0)

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

		requestedPriceString, ok := row[3].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[3]=%v", row[3])
		}

		requestedPrice, parseErr := strconv.ParseFloat(requestedPriceString, 64)
		if parseErr != nil {
			return nil, parseErr
		}

		executedPriceString, ok := row[4].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[4]=%v", row[4])
		}

		executedPrice, parseErr := strconv.ParseFloat(executedPriceString, 64)
		if parseErr != nil {
			return nil, parseErr
		}

		trades = append(trades, &models.Trade{
			Time:           timestamp,
			Symbol:         _symbol,
			Volume:         volume,
			RequestedPrice: requestedPrice,
			ExecutedPrice:  executedPrice,
		})
	}

	return trades, nil
}

func FetchTrades(ctx context.Context) (models.Trades, error) {
	mu.Lock()
	defer mu.Unlock()

	return fetchTrades(ctx, service)
}

func appendTrade(ctx context.Context, srv *sheets.Service, tr *models.Trade) error {
	trades := make(models.Trades, 0)
	trades.Add(tr)
	values := trades.ToRows()
	return appendRows(ctx, srv, spreadsheetId, sheetName, values)
}

func AppendTrade(ctx context.Context, trade *models.Trade) error {
	mu.Lock()
	defer mu.Unlock()

	var trades models.Trades
	trades.Add(trade)
	return appendTrade(ctx, service, trade)
}
