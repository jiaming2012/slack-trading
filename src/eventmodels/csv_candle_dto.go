package eventmodels

import (
	"fmt"
	"log"
	"time"
)

type CsvCandleDTO struct {
	Timestamp       string  `csv:"time"`
	Open            float64 `csv:"open"`
	High            float64 `csv:"high"`
	Low             float64 `csv:"low"`
	Close           float64 `csv:"close"`
	UpTrendBegins   float64 `csv:"UpTrend Begins"`
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

	return &CsvCandle{
		Open:      c.Open,
		High:      c.High,
		Low:       c.Low,
		Close:     c.Close,
		Timestamp: t,
	}
}
