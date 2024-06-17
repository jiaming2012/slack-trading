package run

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	export_data "slack-trading/src/cmd/stats/export_data/run"
	"slack-trading/src/eventmodels"
	"slack-trading/src/utils"
)

type RunArgs struct {
	StartsAt              time.Time
	EndsAt                time.Time
	Ticker                eventmodels.StockSymbol
	LookaheadCandlesCount []int
	GoEnv                 string
}

func Run(args RunArgs) (eventmodels.SignalRunOutput, error) {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return eventmodels.SignalRunOutput{}, fmt.Errorf("missing PROJECTS_DIR environment variable")
	}

	log.Debugf("running supertrend_1h_stoch_rsi_15m_down with args: %v", args)

	// import data
	data := make([]eventmodels.TradingViewCandles, 3)
	durations := []int{15, 60}
	for i, duration := range durations {
		streamName := fmt.Sprintf("candles-%s-%d", strings.ToUpper(string(args.Ticker)), duration)

		output, err := export_data.Run(export_data.RunArgs{
			InputStreamName: streamName,
			StartsAt:        args.StartsAt,
			EndsAt:          args.EndsAt,
			GoEnv:           args.GoEnv,
		})

		if err != nil {
			return eventmodels.SignalRunOutput{}, fmt.Errorf("error exporting data for %v: %v", streamName, err)
		}

		data[i], err = utils.ImportAndSortCandles(output.ExportedFilepath, time.Duration(duration)*time.Minute)
		if err != nil {
			return eventmodels.SignalRunOutput{}, fmt.Errorf("error fetching candles for stream %v: %v", streamName, err)
		}
	}

	// process data
	var candles15 eventmodels.TradingViewCandles = data[0]
	var candles60 eventmodels.TradingViewCandles = data[1]

	log.Infof("processing %d 15m candles", len(candles15))

	signalCount := 0
	for i := 0; i < len(candles15)-1; i++ {
		c1 := candles15[i]
		c2 := candles15[i+1]

		if c1.K > c1.D && c2.K < c2.D && c1.D >= 80 {
			candle60 := candles60.FindClosestCandleBeforeOrAt(c2.Timestamp)

			if candle60.UpTrend > 0 {
				c2.IsSignal = true
				signalCount += 1
			}
		}
	}

	log.Infof("found %d signals", signalCount)

	// Process the candles
	candleDuration := 15 * time.Minute

	// export to csv
	streamName := fmt.Sprintf("candles-%s-15", args.Ticker)
	fname := fmt.Sprintf("%s-from-%s-to-%s", streamName, args.StartsAt.Format("20060102_150405"), args.EndsAt.Format("20060102_150405"))
	outDir := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "transform_data", "supertrend_1h_stoch_rsi_15m_down", "output")
	outDirs, err := utils.ExportToCsv(candles15, args.LookaheadCandlesCount, candleDuration, outDir, fname)

	if err != nil {
		return eventmodels.SignalRunOutput{}, fmt.Errorf("error exporting to csv: %v", err)
	}

	return eventmodels.SignalRunOutput{
		ExportedFilepaths: outDirs,
	}, nil
}
