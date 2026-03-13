package eventmodels

import (
	"fmt"
	"time"
)

type TradingViewCandle struct {
	Timestamp       time.Time
	Open            float64
	High            float64
	Low             float64
	Close           float64
	UpTrend         float64
	UpTrendBegins   float64
	DownTrend       float64
	DownTrendBegins float64
	K               float64
	D               float64
	IsSignal        bool
}

func (c TradingViewCandle) String() string {
	return fmt.Sprintf("TradingViewCandle{Timestamp=%v, Open=%f, High=%f, Low=%f, Close=%f, UpTrend=%f, UpTrendBegins=%f, DownTrend=%f, DownTrendBegins=%f, K=%f, D=%f}", c.Timestamp, c.Open, c.High, c.Low, c.Close, c.UpTrend, c.UpTrendBegins, c.DownTrend, c.DownTrendBegins, c.K, c.D)
}

func (c TradingViewCandle) ToDTO() *TradingViewCandleDTO {
	return &TradingViewCandleDTO{
		Timestamp:       c.Timestamp.Format(time.RFC3339),
		Open:            fmt.Sprintf("%f", c.Open),
		High:            fmt.Sprintf("%f", c.High),
		Low:             fmt.Sprintf("%f", c.Low),
		Close:           fmt.Sprintf("%f", c.Close),
		UpTrend:         fmt.Sprintf("%f", c.UpTrend),
		UpTrendBegins:   fmt.Sprintf("%f", c.UpTrendBegins),
		DownTrend:       fmt.Sprintf("%f", c.DownTrend),
		DownTrendBegins: fmt.Sprintf("%f", c.DownTrendBegins),
		K:               fmt.Sprintf("%f", c.K),
		D:               fmt.Sprintf("%f", c.D),
	}
}
