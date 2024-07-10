package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
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

func (dto *ThetaDataResponseDTO) ConvertToCandles() ([]*ThetaDataCandleDTO, error) {
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

	candles := make([]*ThetaDataCandleDTO, 0)

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

		count, ok := data[6].(float64)
		if !ok {
			return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: missing format for count, data: %v", data[6])
		}

		date, ok := data[7].(float64)
		if !ok {
			return nil, fmt.Errorf("ThetaDataResponseDTO.ConvertToCandles: missing format for date, data: %v", data[7])
		}

		candle := &ThetaDataCandleDTO{
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

type ThetaDataCandleDTO struct {
	MillisecondOfDay float64
	Open             float64
	High             float64
	Low              float64
	Close            float64
	Volume           float64
	Count            float64
	Date             float64
}

func (c *ThetaDataCandleDTO) ToCandleDTO() (eventmodels.CandleDTO, error) {
	date, err := convertDateAndMsToTime(c.Date, c.MillisecondOfDay)
	if err != nil {
		return eventmodels.CandleDTO{}, fmt.Errorf("ThetaDataCandleDTO:  %w", err)
	}

	return eventmodels.CandleDTO{
		Date:   date.Format("2006-01-02 15:04"),
		Open:   c.Open,
		High:   c.High,
		Low:    c.Low,
		Close:  c.Close,
		Volume: int(c.Volume),
	}, nil
}

func convertDateAndMsToTime(date float64, msOfDay float64) (time.Time, error) {
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

func FetchThetaDataHistOptionOHLC(baseURL string, root eventmodels.StockSymbol, optionType eventmodels.OptionType, expiration time.Time, startDate time.Time, endDate time.Time, interval time.Duration, strike float64) (ThetaDataResponseDTO, error) {
	var result ThetaDataResponseDTO

	var right string
	switch optionType {
	case eventmodels.OptionTypeCall:
		right = "C"
	case eventmodels.OptionTypePut:
		right = "P"
	default:
		return result, fmt.Errorf("FetchThetaDataOHLC: invalid option type: %v", optionType)
	}

	expirationStr := expiration.Format("20060102")
	startDateStr := startDate.Format("20060102")
	endDateStr := endDate.Format("20060102")

	intervalM := interval / time.Millisecond

	// Validate interval value
	if intervalM < 100 || intervalM > 3600000 {
		return result, fmt.Errorf("FetchThetaDataOHLC: invalid interval value: %d. Must be between 100 and 3600000 milliseconds", intervalM)
	}

	// Convert ivl (time.Duration) to milliseconds
	intervalStr := fmt.Sprintf("%d", intervalM)

	// Convert strike price to 1/10ths of a cent and to integer
	strikeInt := int(strike * 1000)

	// Define the request URL
	url := fmt.Sprintf("%s/v2/hist/option/ohlc?right=%s&exp=%s&start_date=%s&end_date=%s&root=%s&ivl=%s&strike=%d", baseURL, right, expirationStr, startDateStr, endDateStr, root, intervalStr, strikeInt)

	log.Infof("FetchThetaDataOHLC: fetching data from %s", url)

	// Create a new HTTP GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return result, fmt.Errorf("FetchThetaDataOHLC: %w", err)
	}

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("FetchThetaDataOHLC.Do: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("FetchThetaDataOHLC.ReadAll: %w", err)
	}

	// Unmarshal the JSON response into the result struct
	err = json.Unmarshal(body, &result)
	if err != nil {
		return result, fmt.Errorf("FetchThetaDataOHLC.Unmarshal: %w", err)
	}

	return result, nil
}

func main() {
	baseURL := "http://192.168.1.160:25510"
	symbol := eventmodels.StockSymbol("IWM")
	expiration := time.Date(2024, 7, 10, 0, 0, 0, 0, time.UTC)
	startDate := time.Date(2024, 7, 9, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 7, 10, 0, 0, 0, 0, time.UTC)
	interval := 1 * time.Minute
	strike := 201.00

	response, err := FetchThetaDataHistOptionOHLC(baseURL, symbol, eventmodels.OptionTypeCall, expiration, startDate, endDate, interval, strike)
	if err != nil {
		log.Fatalf("FetchThetaDataOHLC: %v", err)
	}

	candlesDTO, err := response.ConvertToCandles()
	if err != nil {
		log.Fatalf("ConvertToCandles: %v", err)
	}

	log.Infof("Fetched %d candles\n", len(candlesDTO))

	var candles []eventmodels.CandleDTO
	for _, candleDTO := range candlesDTO {
		candle, err := candleDTO.ToCandleDTO()
		if err != nil {
			log.Fatalf("ToCandleDTO: %v", err)
		}

		candles = append(candles, candle)
	}

	for _, candle := range candles {
		log.Infof("Candle: %+v\n", candle)
	}

	log.Infof("Converted %d candles\n", len(candlesDTO))
}
