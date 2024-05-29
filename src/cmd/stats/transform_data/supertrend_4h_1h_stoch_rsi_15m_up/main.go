package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gocarina/gocsv"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/utils"
)

func fetchCandles(inDir string) (eventmodels.TradingViewCandles, error) {
	f, err := os.Open(inDir)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	defer f.Close()

	r := csv.NewReader(f)

	var candlesDTO eventmodels.TradingViewCandlesDTO

	gocsv.UnmarshalCSV(r, &candlesDTO)

	candles := candlesDTO.ToModel()

	candlesSorted := utils.SortCandles(candles, time.Minute*720)

	if err := candlesSorted.Validate(); err != nil {
		return nil, fmt.Errorf("error validating candles: %v", err)
	}

	return candlesSorted, nil
}

func main() {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		panic("missing PROJECTS_DIR environment variable")
	}

	// fetch 15m candles
	fName := "candles-SPX-15.csv"
	inDir := filepath.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "data", fName)
	candles15, err := fetchCandles(inDir)
	if err != nil {
		log.Fatalf("error fetching candles (tf=15): %v", err)
	}

	// fetch 1h candles
	fName = "candles-SPX-60.csv"
	inDir = filepath.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "data", fName)
	candles60, err := fetchCandles(inDir)
	if err != nil {
		log.Fatalf("error fetching candles (tf=60): %v", err)
	}

	// fetch 4h candles
	fName = "candles-SPX-240.csv"
	inDir = filepath.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "data", fName)
	candles240, err := fetchCandles(inDir)
	if err != nil {
		log.Fatalf("error fetching candles (tf=240): %v", err)
	}

	signalCount := 0
	for i := 0; i < len(candles15)-1; i++ {
		c1 := candles15[i]
		c2 := candles15[i+1]

		if c1.K < c1.D && c2.K > c2.D && c1.D <= 20 {
			candle60 := candles60.FindClosestCandleBeforeOrAt(c2.Timestamp)
			candle240 := candles240.FindClosestCandleBeforeOrAt(c2.Timestamp)

			if candle60.UpTrend > 0 && candle240.UpTrend > 0 {
				c2.IsSignal = true
				signalCount += 1
			}
		}
	}

	log.Infof("15m candles: %d", len(candles15))
	log.Infof("found %d signals", signalCount)

	// Process the candles
	candleDuration := 15 * time.Minute
	lookaheadPeriods := []int{4, 8, 16, 24, 96, 192, 288, 480, 672}

	// export to csv
	utils.ExportToCsv(candles15, lookaheadPeriods, candleDuration, "candles-SPX-15")
}
