package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"
)

type StockData struct {
	Time          time.Time
	StockPrice    float64
	TargetPrice   float64
	EventTime     *time.Time
	EventOccurred int
}

func calculateEventOccurred(data []StockData) {
	for i := 0; i < len(data); i++ {
		data[i].EventOccurred = 0

		for j := i + 1; j < len(data); j++ {
			if data[j].StockPrice >= data[i].TargetPrice {
				data[i].EventTime = &data[j].Time
				data[i].EventOccurred = 1

				fmt.Println("Event occurred at", data[i].EventTime.Format("2006-01-02 15:04:05"))
			}
		}
	}
}

func main() {
	file, err := os.Open("stock_data.csv")
	if err != nil {
		fmt.Println("Error opening CSV file:", err)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // see the Reader struct information below

	rawCSVdata, err := reader.ReadAll()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Parameters
	targetPercentage := 0.25

	var stockData []StockData
	for _, each := range rawCSVdata {
		// skip header
		if each[0] == "Time" {
			continue
		}

		stockPrice, _ := strconv.ParseFloat(each[1], 64)
		timeValue, _ := time.Parse("2006-01-02 15:04:05", each[0])
		targetPrice := stockPrice * (1 + targetPercentage)

		stockData = append(stockData, StockData{Time: timeValue, StockPrice: stockPrice, TargetPrice: targetPrice})
	}

	calculateEventOccurred(stockData)

	// Export the data
	file, err = os.Create("stock_data_clean.csv")
	if err != nil {
		fmt.Println("Error creating CSV file:", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"Signal Time", "Stock Price", "Target Price", "Event Time", "Event Occurred"})

	for _, data := range stockData {
		eventTime := ""
		if data.EventOccurred == 1 {
			eventTime = data.EventTime.Format("2006-01-02 15:04:05")
		}

		writer.Write([]string{data.Time.Format("2006-01-02 15:04:05"), fmt.Sprintf("%.2f", data.StockPrice), fmt.Sprintf("%.2f", data.TargetPrice), eventTime, fmt.Sprintf("%d", data.EventOccurred)})
	}
}
