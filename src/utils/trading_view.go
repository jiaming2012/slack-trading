package utils

import (
	"time"

	"slack-trading/src/eventmodels"
)

func IsMarkedClosed(candle *eventmodels.TradingViewCandle) bool {
	// no if saturday or sunday
	if candle.Timestamp.Weekday() == time.Saturday || candle.Timestamp.Weekday() == time.Sunday {
		return true
	}

	// yes if between 19:55:00 +0000 UTC and 13:30:00 +0000 UTC
	// no otherwise
	if candle.Timestamp.Hour() == 19 && candle.Timestamp.Minute() >= 55 {
		return true
	}

	if candle.Timestamp.Hour() == 20 {
		return true
	}

	if candle.Timestamp.Hour() == 13 && candle.Timestamp.Minute() < 30 {
		return true
	}

	return false
}
