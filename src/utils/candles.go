package utils

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/gocarina/gocsv"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func ReverseCandlesDTO(candles []*eventmodels.CandleDTO) []*eventmodels.CandleDTO {
	for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
		candles[i], candles[j] = candles[j], candles[i]
	}
	return candles
}

func ImportAndSortCandles(inDir string, timeframe time.Duration) (eventmodels.TradingViewCandles, error) {
	f, err := os.Open(inDir)
	if err != nil {
		return eventmodels.TradingViewCandles{}, fmt.Errorf("error opening file: %v", err)
	}

	defer f.Close()

	r := csv.NewReader(f)

	var candlesDTO eventmodels.TradingViewCandlesDTO

	gocsv.UnmarshalCSV(r, &candlesDTO)

	candles := candlesDTO.ToModel()

	candlesSorted := SortCandles(candles, timeframe)

	if err := candlesSorted.Validate(); err != nil {
		return nil, fmt.Errorf("error validating candles: %v", err)
	}

	return candlesSorted, nil
}

func SortCandles(candles eventmodels.TradingViewCandles, timeFrame time.Duration) eventmodels.TradingViewCandles {
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
		if candlesNoDuplicates[i].Timestamp.Add(timeFrame).Before(candlesNoDuplicates[i+1].Timestamp) {
			if !IsMarkedClosed(candlesNoDuplicates[i].Timestamp.Add(timeFrame)) {
				log.Warnf("SortCandles: Gap of %v data between %v and %v", timeFrame, candlesNoDuplicates[i].Timestamp, candlesNoDuplicates[i+1].Timestamp)
			}
		}
	}

	return candlesNoDuplicates
}
