package eventmodels

import (
	"fmt"
	"strconv"
	"time"
)

type OandaFetchQuotesResponseDTO struct {
	Instrument  string `json:"instrument"`
	Granularity string `json:"granularity"`
	Candles     []struct {
		Complete bool      `json:"complete"`
		Volume   int       `json:"volume"`
		Time     time.Time `json:"time"`
		Mid      struct {
			Open  string `json:"o"`
			High  string `json:"h"`
			Low   string `json:"l"`
			Close string `json:"c"`
		} `json:"mid"`
	} `json:"candles"`
}

func (r *OandaFetchQuotesResponseDTO) GetLastCandle(now time.Time) (*Candle, error) {
	if len(r.Candles) == 0 {
		return nil, fmt.Errorf("OandaFetchQuotesResponseDTO: No candles found")
	}

	lastCandle := r.Candles[len(r.Candles)-1]

	open, err := strconv.ParseFloat(lastCandle.Mid.Open, 64)
	if err != nil {
		return nil, fmt.Errorf("OandaFetchQuotesResponseDTO: Failed to parse open price: %v", err)
	}

	high, err := strconv.ParseFloat(lastCandle.Mid.High, 64)
	if err != nil {
		return nil, fmt.Errorf("OandaFetchQuotesResponseDTO: Failed to parse high price: %v", err)
	}

	low, err := strconv.ParseFloat(lastCandle.Mid.Low, 64)
	if err != nil {
		return nil, fmt.Errorf("OandaFetchQuotesResponseDTO: Failed to parse low price: %v", err)
	}

	close, err := strconv.ParseFloat(lastCandle.Mid.Close, 64)
	if err != nil {
		return nil, fmt.Errorf("OandaFetchQuotesResponseDTO: Failed to parse close price: %v", err)
	}

	return &Candle{
		Timestamp:   lastCandle.Time,
		LastUpdated: now,
		Open:        open,
		High:        high,
		Low:         low,
		Close:       close,
	}, nil
}
