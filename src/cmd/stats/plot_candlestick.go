package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
)

// Define the structure for the input data
type InputData struct {
	CandleData struct {
		Date  []string  `json:"Date"`
		Open  []float64 `json:"Open"`
		High  []float64 `json:"High"`
		Low   []float64 `json:"Low"`
		Close []float64 `json:"Close"`
	} `json:"candle_data"`
	OrderData struct {
		Date  []string  `json:"Date"`
		Type  []string  `json:"Type"`
		Price []float64 `json:"Price"`
	} `json:"order_data"`
	OptionData struct {
		Date  []string  `json:"Date"`
		Open  []float64 `json:"Open"`
		Close []float64 `json:"Close"`
	} `json:"option_data"`
	StrikePrice float64 `json:"strike_price"`
}

func main() {
	// Sample JSON data
	data := InputData{
		CandleData: struct {
			Date  []string  `json:"Date"`
			Open  []float64 `json:"Open"`
			High  []float64 `json:"High"`
			Low   []float64 `json:"Low"`
			Close []float64 `json:"Close"`
		}{
			Date:  []string{"2024-06-01 09:00", "2024-06-01 09:15", "2024-06-01 09:30", "2024-06-01 09:45"},
			Open:  []float64{100, 101, 102, 103},
			High:  []float64{105, 106, 107, 108},
			Low:   []float64{95, 96, 97, 98},
			Close: []float64{103, 99, 105, 108},
		},
		OrderData: struct {
			Date  []string  `json:"Date"`
			Type  []string  `json:"Type"`
			Price []float64 `json:"Price"`
		}{
			Date:  []string{"2024-06-01 09:15", "2024-06-01 9:45"},
			Type:  []string{"Sell", "Buy"},
			Price: []float64{105, 108},
		},
		OptionData: struct {
			Date  []string  `json:"Date"`
			Open  []float64 `json:"Open"`
			Close []float64 `json:"Close"`
		}{
			Date:  []string{"2024-06-01 09:00", "2024-06-01 09:15", "2024-06-01 09:30", "2024-06-01 09:45"},
			Open:  []float64{14, 11, 12, 13},
			Close: []float64{14, 13, 14, 15},
		},
		StrikePrice: 104,
	}

	// Convert the data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Error marshalling JSON: %v", err)
	}

	// Prepare the command to run the Python script
	cmd := exec.Command("python3", "plot_candlestick.py", string(jsonData))

	// Set the standard input to the JSON data
	cmd.Stdin = bytes.NewReader(jsonData)

	// Capture the output
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	// Run the command
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error running Python script: %v\nOutput: %s", err, out.String())
	}

	// Print the output
	fmt.Println(out.String())
}
