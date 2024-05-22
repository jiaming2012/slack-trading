package eventmodels

import (
	"fmt"
	"log"
	"math"
	"time"
)

type CsvCandleDTO struct {
	Timestamp       string  `csv:"time"`
	Open            float64 `csv:"open"`
	High            float64 `csv:"high"`
	Low             float64 `csv:"low"`
	Close           float64 `csv:"close"`
	UpTrend         float64 `csv:"Up Trend"`
	UpTrendBegins   float64 `csv:"UpTrend Begins"`
	DownTrend       float64 `csv:"Down Trend"`
	DownTrendBegins float64 `csv:"DownTrend Begins"`
}

func (c *CsvCandleDTO) ToModel() *CsvCandle {
	t, err := time.Parse(time.RFC3339, c.Timestamp)
	if err != nil {
		t, err = time.Parse("2006-01-02", c.Timestamp)
		if err != nil {
			log.Fatal(fmt.Errorf("error parsing time: %v", err))
		}
	}

	if math.IsNaN(c.UpTrend) {
		c.UpTrend = 0
	}

	if math.IsNaN(c.DownTrend) {
		c.DownTrend = 0
	}

	if math.IsNaN(c.UpTrendBegins) {
		c.UpTrendBegins = 0
	}

	if math.IsNaN(c.DownTrendBegins) {
		c.DownTrendBegins = 0
	}

	return &CsvCandle{
		Open:            c.Open,
		High:            c.High,
		Low:             c.Low,
		Close:           c.Close,
		UpTrend:         c.UpTrend,
		DownTrend:       c.DownTrend,
		UpTrendBegins:   c.UpTrendBegins,
		DownTrendBegins: c.DownTrendBegins,
		Timestamp:       t,
	}
}
