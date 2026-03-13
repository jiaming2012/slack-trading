package main

import (
	"fmt"
	"github.com/sajari/regression"
)

type Candlestick struct {
	Time  int
	High  float64
	Low   float64
	Close float64
	Open  float64
}

func GenerateTrendline(candlesticks []Candlestick, windowSize int) ([]float64, error) {
	xValues := make([]float64, 0)
	yValues := make([]float64, 0)

	for i := windowSize - 1; i < len(candlesticks); i++ {
		sum := 0.0
		for j := i - windowSize + 1; j <= i; j++ {
			sum += candlesticks[j].Close
		}
		average := sum / float64(windowSize)
		xValues = append(xValues, float64(candlesticks[i].Time))
		yValues = append(yValues, average)
	}

	r := new(regression.Regression)
	r.SetObserved("Y")
	r.SetVar(0, "X")

	for i, x := range xValues {
		r.Train(regression.DataPoint(yValues[i], []float64{x}))
	}

	r.Run()

	trendline := make([]float64, 0)
	for _, x := range xValues {
		predict, err := r.Predict([]float64{x})
		if err != nil {
			panic(err)
		}

		trendline = append(trendline, predict)
	}

	return trendline, nil
}

func main() {
	candlesticks := []Candlestick{
		{1, 15.0, 12.0, 13.0, 14.0},
		{2, 16.0, 13.0, 14.0, 15.0},
		{3, 17.0, 14.0, 15.0, 16.0},
		{4, 16.0, 13.0, 14.0, 15.0},
		{5, 15.0, 12.0, 13.0, 14.0},
		{6, 14.0, 11.0, 12.0, 13.0},
	}

	windowSize := 4

	trendline, err := GenerateTrendline(candlesticks, windowSize)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	for i, value := range trendline {
		fmt.Printf("Trendline[%d]: %.2f\n", i, value)
	}
}
