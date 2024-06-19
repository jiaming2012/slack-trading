package utils

import (
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func SortCandlesAndMarkSignals(candles eventmodels.TradingViewCandles, candleDuration time.Duration, signalCondition func(candle *eventmodels.TradingViewCandle) bool) []*eventmodels.TradingViewCandle {
	candles = SortCandles(candles, candleDuration)

	signalCount := 0

	for _, candle := range candles {
		if signalCondition(candle) {
			candle.IsSignal = true
			signalCount += 1
		} else {
			candle.IsSignal = false
		}
	}

	log.Infof("found %d signals", signalCount)

	return candles
}
