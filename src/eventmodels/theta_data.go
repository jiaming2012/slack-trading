package eventmodels

import (
	"fmt"
	"strconv"
	"time"
)

type ThetaDataResponseHeaderDTO struct {
	LatencyMs int      `json:"latency_ms"`
	NextPage  string   `json:"next_page"`
	Format    []string `json:"format"`
}

// Define the struct to match the JSON structure
type ThetaDataResponseDTO struct {
	Header   ThetaDataResponseHeaderDTO `json:"header"`
	Response [][]interface{}            `json:"response"`
}

func (dto *ThetaDataResponseDTO) ConvertToCandles() ([]*ThetaDataCandle, error) {
	if len(dto.Header.Format) != 8 {
		return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: invalid format length, expected 8, got %d", len(dto.Header.Format))
	}

	if dto.Header.Format[0] != "ms_of_day" {
		return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: invalid format for millisecond of day, expected 'ms_of_day', got '%s'", dto.Header.Format[0])
	}

	if dto.Header.Format[1] != "open" {
		return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: invalid format for open, expected 'open', got '%s'", dto.Header.Format[1])
	}

	if dto.Header.Format[2] != "high" {
		return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: invalid format for high, expected 'high', got '%s'", dto.Header.Format[2])
	}

	if dto.Header.Format[3] != "low" {
		return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: invalid format for low, expected 'low', got '%s'", dto.Header.Format[3])
	}

	if dto.Header.Format[4] != "close" {
		return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: invalid format for close, expected 'close', got '%s'", dto.Header.Format[4])
	}

	if dto.Header.Format[5] != "volume" {
		return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: invalid format for volume, expected 'volume', got '%s'", dto.Header.Format[5])
	}

	if dto.Header.Format[6] != "count" {
		return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: invalid format for count, expected 'count', got '%s'", dto.Header.Format[6])
	}

	if dto.Header.Format[7] != "date" {
		return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: invalid format for date, expected 'date', got '%s'", dto.Header.Format[7])
	}

	candles := make([]*ThetaDataCandle, 0)

	for _, data := range dto.Response {
		msOfDay, ok := data[0].(float64)
		if !ok {
			return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: missing format for millisecond of day, data: %v", data[0])
		}

		open, ok := data[1].(float64)
		if !ok {
			return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: missing format for open, data: %v", data[1])
		}

		high, ok := data[2].(float64)
		if !ok {
			return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: missing format for high, data: %v", data[2])
		}

		low, ok := data[3].(float64)
		if !ok {
			return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: missing format for low, data: %v", data[3])
		}

		close, ok := data[4].(float64)
		if !ok {
			return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: missing format for close, data: %v", data[4])
		}

		volume, ok := data[5].(float64)
		if !ok {
			return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: missing format for volume, data: %v", data[5])
		}

		countFloat, ok := data[6].(float64)
		if !ok {
			return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: missing format for count, data: %v", data[6])
		}

		count := int(countFloat)

		date, ok := data[7].(float64)
		if !ok {
			return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: missing format for date, data: %v", data[7])
		}

		candle := &ThetaDataCandle{
			MillisecondOfDay: msOfDay,
			Open:             open,
			High:             high,
			Low:              low,
			Close:            close,
			Volume:           volume,
			Count:            count,
			Date:             date,
		}

		candles = append(candles, candle)
	}

	return candles, nil
}

type ThetaDataCandle struct {
	MillisecondOfDay float64
	Open             float64
	High             float64
	Low              float64
	Close            float64
	Volume           float64
	Count            int
	Date             float64
}

func (c *ThetaDataCandle) convertDateAndMsToTime(date float64, msOfDay float64) (time.Time, error) {
	// Parse the date
	dateStr := fmt.Sprintf("%.0f", date)
	year, err := strconv.Atoi(dateStr[:4])
	if err != nil {
		return time.Time{}, fmt.Errorf("convertDateAndMsToTime: failed to convert year: %w", err)
	}
	month, err := strconv.Atoi(dateStr[4:6])
	if err != nil {
		return time.Time{}, fmt.Errorf("convertDateAndMsToTime: failed to convert month: %w", err)
	}
	day, err := strconv.Atoi(dateStr[6:])
	if err != nil {
		return time.Time{}, fmt.Errorf("convertDateAndMsToTime: failed to convert day: %w", err)
	}

	// Convert milliseconds to hours, minutes, and seconds
	seconds := int(msOfDay) / 1000
	msRemaining := int(msOfDay) % 1000
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	// Combine the date and time components
	return time.Date(year, time.Month(month), day, hours, minutes, secs, msRemaining*1e6, time.UTC), nil
}

func (c *ThetaDataCandle) ToCandleDTO() (CandleDTO, error) {
	date, err := c.convertDateAndMsToTime(c.Date, c.MillisecondOfDay)
	if err != nil {
		return CandleDTO{}, fmt.Errorf("ThetaDataCandleDTO:  %w", err)
	}

	return CandleDTO{
		Date:   date.Format("2006-01-02 15:04"),
		Open:   c.Open,
		High:   c.High,
		Low:    c.Low,
		Close:  c.Close,
		Volume: int(c.Volume),
	}, nil
}
