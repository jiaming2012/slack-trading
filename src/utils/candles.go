package utils

import (
	"sort"
	"time"

	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
)

func SortCandles(candles eventmodels.TradingViewCandles, timeFrameInMinutes time.Duration) eventmodels.TradingViewCandles {
	xValues := map[time.Time]*eventmodels.TradingViewCandle{}

	// remove duplicates
	for _, candle := range candles {
		xValues[candle.Timestamp] = candle
	}

	var candlesNoDuplicates []*eventmodels.TradingViewCandle
	for _, candle := range xValues {
		candlesNoDuplicates = append(candlesNoDuplicates, candle)
	}

	// sort candlesNoDuplicates by time
	sort.Slice(candlesNoDuplicates, func(i, j int) bool {
		return candlesNoDuplicates[i].Timestamp.Before(candlesNoDuplicates[j].Timestamp)
	})

	// check for gaps in the data
	for i := 0; i < len(candlesNoDuplicates)-1; i++ {
		if candlesNoDuplicates[i].Timestamp.Add(timeFrameInMinutes).Before(candlesNoDuplicates[i+1].Timestamp) {
			if !IsMarkedClosed(candlesNoDuplicates[i]) {
				log.Warnf("Gap in data between %v and %v", candlesNoDuplicates[i].Timestamp, candlesNoDuplicates[i+1].Timestamp)
			}
		}
	}

	return candlesNoDuplicates
}
