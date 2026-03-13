package models

import (
	"fmt"
	"strconv"
	"time"
)

type Row []interface{}
type Rows [][]interface{}

func (r Rows) ConvertToCandles() (*Candles, error) {
	candles := Candles{
		Period: 5,
	}

	for _, row := range r {
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

		openPriceStr, ok := row[2].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[2]=%v", row[2])
		}

		openPrice, parseErr := strconv.ParseFloat(openPriceStr, 64)
		if parseErr != nil {
			return nil, parseErr
		}

		highPriceStr, ok := row[3].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[3]=%v", row[3])
		}

		highPrice, parseErr := strconv.ParseFloat(highPriceStr, 64)
		if parseErr != nil {
			return nil, parseErr
		}

		lowPriceStr, ok := row[4].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[4]=%v", row[4])
		}

		lowPrice, parseErr := strconv.ParseFloat(lowPriceStr, 64)
		if parseErr != nil {
			return nil, parseErr
		}

		closePriceStr, ok := row[5].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse row[5]=%v", row[5])
		}

		closePrice, parseErr := strconv.ParseFloat(closePriceStr, 64)
		if parseErr != nil {
			return nil, parseErr
		}

		candles.Data = append(candles.Data, Candle{
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