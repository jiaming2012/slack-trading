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
