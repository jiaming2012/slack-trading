package utils

import (
	"encoding/csv"
	"fmt"
	"os"
	"path"
	"time"

	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
)

type percentChangeData struct {
	Time          time.Time
	PercentChange float64
}

func findPercentChange(candles []*eventmodels.TradingViewCandle, index, lookahead int) float64 {
	if index+lookahead >= len(candles) {
		return (candles[len(candles)-1].Close - candles[index].Close) / candles[index].Close * 100
	}

	return (candles[index+lookahead].Close - candles[index].Close) / candles[index].Close * 100
}

func ExportToCsv(candles []*eventmodels.TradingViewCandle, lookaheadPeriods []int, candleDuration time.Duration, outDir string, fname string) ([]string, error) {
	data := make(map[int][]percentChangeData)

	for index, c := range candles {
		if c.IsSignal {
			for _, lookahead := range lookaheadPeriods {
				data[lookahead] = append(data[lookahead], percentChangeData{
					Time:          c.Timestamp,
					PercentChange: findPercentChange(candles, index, lookahead),
				})
			}
		}
	}

	output := []string{}

	for lookahead, percentChanges := range data {
		// Create export directory
		if _, err := os.Stat(outDir); os.IsNotExist(err) {
			os.Mkdir(outDir, 0755)
		}

		// Export the data
		lookaheadLabel := fmt.Sprintf("%d", lookahead)
		outFile := path.Join(outDir, fmt.Sprintf("percent_change-%s-lookahead-%s.csv", fname, lookaheadLabel))
		if _, err := os.Stat(outFile); err == nil {
			log.Infof("Data file %s already exists", outFile)
			output = append(output, outFile)
			continue
		}

		file, err := os.Create(outFile)
		if err != nil {
			return nil, fmt.Errorf("error creating CSV file: %v", err)
		}

		defer file.Close()

		writer := csv.NewWriter(file)
		defer writer.Flush()

		// Write header
		writer.Write([]string{"Time", "Percent_Change"})

		for _, data := range percentChanges {
			timeInISO := data.Time.Format(time.RFC3339)
			writer.Write([]string{timeInISO, fmt.Sprintf("%f", data.PercentChange)})
		}

		output = append(output, outFile)

		log.Infof("Exported %d percent change rows to %s", len(percentChanges), outFile)
	}

	return output, nil
}
