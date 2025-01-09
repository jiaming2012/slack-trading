package eventmodels

import (
	"fmt"
	"math"
	"sort"
	"time"
)

type CandleDTO struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Vwap   float64 `json:"vwap"`
	Volume float64 `json:"volume"`
}

func (c *CandleDTO) ToCandle(loc *time.Location) (Candle, error) {
	layout := "2006-01-02 15:04:05" // Go's reference time format
	date, err := time.ParseInLocation(layout, c.Date, loc)
	if err != nil {
		return Candle{}, fmt.Errorf("ToCandle: error parsing date: %v", err)
	}

	return Candle{
		Timestamp:   date,
		LastUpdated: date,
		Open:        c.Open,
		High:        c.High,
		Low:         c.Low,
		Close:       c.Close,
		Volume:      c.Volume,
		Vwap:        c.Vwap,
	}, nil
}

type CandleDTOs []*CandleDTO

func (c CandleDTOs) ConvertToCandleData() CandleData {
	var date []string
	var open []float64
	var high []float64
	var low []float64
	var close []float64
	for _, candle := range c {
		date = append(date, candle.Date)
		open = append(open, candle.Open)
		high = append(high, candle.High)
		low = append(low, candle.Low)
		close = append(close, candle.Close)
	}
	return CandleData{
		Date:  date,
		Open:  open,
		High:  high,
		Low:   low,
		Close: close,
	}
}

type CandleSpread struct {
	Candle1 CandleDTO
	Candle2 CandleDTO
}

func DeriveSpreadCandles(candles1 []CandleDTO, candles2 []CandleDTO) []*CandleDTO {
	data := make(map[string]CandleSpread)
	for _, c1 := range candles1 {
		data[c1.Date] = CandleSpread{Candle1: c1}
	}

	for _, c2 := range candles2 {
		if spread, ok := data[c2.Date]; ok {
			spread.Candle2 = c2
			data[c2.Date] = spread
		} else {
			data[c2.Date] = CandleSpread{Candle2: c2}
		}
	}

	// sort the data by date
	var datesSorted []string
	for date := range data {
		datesSorted = append(datesSorted, date)
	}

	// sort the dates
	sort.Strings(datesSorted)

	var out []*CandleDTO
	for _, date := range datesSorted {
		spread := data[date]
		if spread.Candle1.Date != "" && spread.Candle2.Date != "" {
			out = append(out, &CandleDTO{
				Date: date,
				// Open:   spread.Candle1.Open - spread.Candle2.Open,
				// High:   spread.Candle1.High - spread.Candle2.High,
				// Low:    spread.Candle1.Low - spread.Candle2.Low,
				// Close:  spread.Candle1.Close - spread.Candle2.Close,
				Open:   math.Abs(spread.Candle1.Open - spread.Candle2.Open),
				High:   math.Abs(spread.Candle1.High - spread.Candle2.High),
				Low:    math.Abs(spread.Candle1.Low - spread.Candle2.Low),
				Close:  math.Abs(spread.Candle1.Close - spread.Candle2.Close),
				Volume: spread.Candle1.Volume + spread.Candle2.Volume,
			})
		}
	}

	return out
}
