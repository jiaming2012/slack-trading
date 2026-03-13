package main

import (
	"log"
	"os"

	"github.com/jiaming2012/slack-trading/src/cmd/stats/plot_candlestick/run"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func main() {
	// Sample JSON data
	data := eventmodels.PlotOrderInputData{
		CandleData: eventmodels.CandleData{
			Date:  []string{"2024-06-01 09:00", "2024-06-01 09:15", "2024-06-01 09:30", "2024-06-01 09:45"},
			Open:  []float64{100, 101, 102, 103},
			High:  []float64{105, 106, 107, 108},
			Low:   []float64{95, 96, 97, 98},
			Close: []float64{103, 99, 105, 108},
		},
		OrderData: eventmodels.OrderData{
			Date:         []string{"2024-06-01 09:15", "2024-06-01 9:45"},
			Type:         []string{"Sell", "Buy"},
			Price:        []float64{105, 108},
			StrikePriceA: 104,
			StrikePriceB: 107,
		},
		OptionData: eventmodels.CandleData{
			Date:  []string{"2024-06-01 09:00", "2024-06-01 09:15", "2024-06-01 09:30", "2024-06-01 09:45"},
			Open:  []float64{14, 11, 12, 13},
			High:  []float64{15, 12, 13, 14},
			Low:   []float64{13, 10, 11, 12},
			Close: []float64{14, 13, 14, 15},
		},
	}

	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		log.Fatalf("missing PROJECTS_DIR environment variable")
	}

	output, err := run.ExecPlotCandlestick(projectsDir, data)
	if err != nil {
		log.Fatalf("ExecPlotCandlestick Error: %v", err)
	}

	log.Printf("Output: %s", output)
}
