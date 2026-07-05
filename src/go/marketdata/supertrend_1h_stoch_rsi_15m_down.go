package marketdata

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/utils"
)

func Run_SuperTrend1hStochRsi15mDown(args models.SupertrendRunArgs) (models.SignalRunOutput, error) {
	projectDir := os.Getenv("TRADING_PROJECT_DIR")
	if projectDir == "" {
		return models.SignalRunOutput{}, fmt.Errorf("missing TRADING_PROJECT_DIR environment variable")
	}

	log.Debugf("running supertrend_1h_stoch_rsi_15m_down with args: %v", args)

	// import data
	data := make([]models.TradingViewCandles, 3)
	durations := []int{15, 60}
	for i, duration := range durations {
		streamName := fmt.Sprintf("candles-%s-%d", strings.ToUpper(string(args.Ticker)), duration)

		output, err := ExportData(models.ExportDataRunArgs{
			InputStreamName: streamName,
			StartsAt:        args.StartsAt,
			EndsAt:          args.EndsAt,
			GoEnv:           args.GoEnv,
		})

		if err != nil {
			return models.SignalRunOutput{}, fmt.Errorf("error exporting data for %v: %v", streamName, err)
		}

		data[i], err = models.ImportAndSortCandles(output.ExportedFilepath, time.Duration(duration)*time.Minute)
		if err != nil {
			return models.SignalRunOutput{}, fmt.Errorf("error fetching candles for stream %v: %v", streamName, err)
		}
	}

	// process data
	var candles15 models.TradingViewCandles = data[0]
	var candles60 models.TradingViewCandles = data[1]

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
	outDir := path.Join(projectDir, "src", "cmd", "stats", "transform_data", "supertrend_1h_stoch_rsi_15m_down", "output")
	outDirs, err := utils.ExportToCsv(candles15, args.LookaheadCandlesCount, candleDuration, outDir, fname)

	if err != nil {
		return models.SignalRunOutput{}, fmt.Errorf("error exporting to csv: %v", err)
	}

	return models.SignalRunOutput{
		ExportedFilepaths: outDirs,
	}, nil
}
