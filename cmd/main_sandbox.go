package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"slack-trading/src/models"
	"slack-trading/src/sheets"
)

type LevelInfo struct {
	Level float64 `json:"level"`
	Tests int     `json:"tests"`
}

type LevelInfos []LevelInfo

func (l LevelInfos) Len() int {
	return len(l)
}

func (l LevelInfos) Less(i, j int) bool {
	return l[i].Tests < l[j].Tests
}

func (l LevelInfos) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

type JSONResponse struct {
	Support    []LevelInfo `json:"support"`
	Resistance []LevelInfo `json:"resistance"`
}

func calculateMedian(data []float64) float64 {
	n := len(data)
	if n == 0 {
		return 0.0
	}

	// Sort the data in ascending order
	sort.Float64s(data)

	// Calculate the median
	if n%2 == 0 {
		return (data[n/2-1] + data[n/2]) / 2
	}
	return data[n/2]
}

func calculateStdDev(data []float64) float64 {
	n := len(data)
	if n < 2 {
		return 0.0
	}

	// Calculate the mean
	sum := 0.0
	for _, value := range data {
		sum += value
	}
	mean := sum / float64(n)

	// Calculate the sum of squared differences from the mean
	sumSquaredDiff := 0.0
	for _, value := range data {
		diff := value - mean
		sumSquaredDiff += diff * diff
	}

	// Calculate the standard deviation
	return math.Sqrt(sumSquaredDiff / float64(n-1))
}

func calculateMinMax(numbers []float64) (float64, float64) {
	min := math.Inf(1)
	max := math.Inf(-1)

	for _, num := range numbers {
		if num < min {
			min = num
		}
		if num > max {
			max = num
		}
	}

	return min, max
}

func makeToleranceFunc(prices []float64) func(float64, float64) bool {

	// Sort the training data in ascending order
	sort.Float64s(prices)

	// Calculate the median of the training data
	median := calculateMedian(prices)

	// Calculate the standard deviation of the training data
	//stdDev := calculateStdDev(prices)

	// Determine the tolerance based on heuristics
	//tolerance := stdDev / 4
	min, max := calculateMinMax(prices)
	tolerance := (max - min) / 20
	tolerance = 5

	fmt.Printf("DEBUG calculated median %f, tolerance %f\n", median, tolerance)

	f := func(p1 float64, p2 float64) bool {
		return math.Abs(p1-median) <= tolerance && math.Abs(p2-median) <= tolerance
	}

	return f
}

func calculateLevels(candles *models.Candles, levelType string, minimumTouches int, numberOfLevels int) []LevelInfo {
	levels := make(map[float64]int)
	supportValues := make([]float64, 0)
	resistanceValues := make([]float64, 0)

	for _, c := range candles.Data {
		supportValues = append(supportValues, c.Low)
		resistanceValues = append(resistanceValues, c.High)
	}

	var isClose func(float64, float64) bool
	fmt.Printf("DEBUG: making support tolerance function ...\n")
	isCloseToSupport := makeToleranceFunc(supportValues)
	fmt.Printf("DEBUG: making resistance tolerance function ...\n")
	isCloseToResistance := makeToleranceFunc(resistanceValues)

	for _, candlestick := range candles.Data {
		var price float64

		switch levelType {
		case "support":
			price = candlestick.Low
			isClose = isCloseToSupport
		case "resistance":
			price = candlestick.High
			isClose = isCloseToResistance
		}

		for level, _ := range levels {
			if isClose(price, level) {
				fmt.Printf("INFO: adding %s %f to level %f\n", levelType, price, level)
				levels[level]++
			}
		}

		levels[price]++
	}

	var info []LevelInfo
	for price, tests := range levels {
		if tests >= minimumTouches {
			info = append(info, LevelInfo{
				Level: price,
				Tests: tests,
			})
		}
	}

	if numberOfLevels > 0 {
		sort.Sort(LevelInfos(info))
		index := int(math.Min(float64(numberOfLevels), float64(len(info))))
		return info[len(info)-index:]
	}

	return info
}

func main() {
	ctx := context.Background()

	// setup google sheets
	if _, _, err := sheets.NewClientFromEnv(ctx); err != nil {
		panic(fmt.Errorf("failed to initialize google sheets: %v", err))
	}

	// run
	candles, err := sheets.FetchLastXCandles(ctx, 266)
	if err != nil {
		panic(err)
	}

	if candles == nil {
		panic("no candles returned")
	}

	fmt.Println("candlesticks ...")

	for _, c := range candles.Data {
		fmt.Println(c.Open, " ... ", c.High, " ... ", c.Low, " ... ", c.Close)
	}

	minimumTouches := 3
	numberOfLevels := 3
	response := JSONResponse{
		Support:    calculateLevels(candles, "support", minimumTouches, numberOfLevels),
		Resistance: calculateLevels(candles, "resistance", minimumTouches, numberOfLevels),
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

	jsonString := string(jsonBytes)
	fmt.Println(jsonString)
}
