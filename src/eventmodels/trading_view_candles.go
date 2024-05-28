package eventmodels

import (
	"fmt"
	"time"
)

type TradingViewCandles []*TradingViewCandle

func (candles TradingViewCandles) FindClosestCandleBeforeOrAt(timestamp time.Time) *TradingViewCandle {
	var closestCandle *TradingViewCandle
	for _, candle := range candles {
		if candle.Timestamp.After(timestamp) {
			break
		}

		closestCandle = candle
	}

	return closestCandle
}

func (candles TradingViewCandles) Validate() error {
	for _, candle := range candles {
		var prevTimestamp time.Time
		if candle.Timestamp.Before(prevTimestamp) {
			return fmt.Errorf("invalid candle: %v, prevTimestamp=%v", candle, prevTimestamp)
		}

		if candle.UpTrend < 0 && candle.DownTrend < 0 {
			return fmt.Errorf("invalid candle: %v, uptrend=%v, downtrend=%v", candle, candle.UpTrend, candle.DownTrend)
		}
	}

	return nil
}
