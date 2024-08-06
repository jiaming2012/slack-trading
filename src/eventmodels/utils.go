package eventmodels

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gocarina/gocsv"
	log "github.com/sirupsen/logrus"
)

func GetMinTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}

	return b
}

func GetMaxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}

	return b
}

func FormatDuration(d time.Duration) string {
	parts := []string{}

	days := d / (time.Hour * 24)
	d -= days * 24 * time.Hour

	if days > 0 {
		parts = append(parts, fmt.Sprintf("%d days", days))
	}

	hours := d / time.Hour
	d -= hours * time.Hour

	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d hours", hours))
	}

	minutes := d / time.Minute
	d -= minutes * time.Minute

	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%d minutes", minutes))
	}

	seconds := d / time.Second

	if seconds > 0 {
		parts = append(parts, fmt.Sprintf("%d seconds", seconds))
	}

	return strings.Join(parts, ", ")
}

func ConvertToMarketOpen(expiration time.Time) (time.Time, error) {
	// market open is 9:30 AM EST
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to load location: %w", err)
	}

	return time.Date(expiration.Year(), expiration.Month(), expiration.Day(), 9, 30, 0, 0, loc), nil
}

func ConvertToMarketClose(expiration time.Time) (time.Time, error) {
	// market close is 4:00 PM EST
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to load location: %w", err)
	}

	return time.Date(expiration.Year(), expiration.Month(), expiration.Day(), 16, 0, 0, 0, loc), nil
}

func ParseDuration(s string) (int, int, int, error) {
	re := regexp.MustCompile(`(?P<months>\d+m)?(?P<days>\d+d)?(?P<hours>\d+h)?`)
	matches := re.FindStringSubmatch(s)

	months := 0
	days := 0
	hours := 0

	for i, name := range re.SubexpNames() {
		if i != 0 && matches[i] != "" {
			value, err := strconv.Atoi(matches[i][:len(matches[i])-1])
			if err != nil {
				return 0, 0, 0, fmt.Errorf("invalid duration: %s", s)
			}

			switch name {
			case "months":
				months = value
			case "days":
				days = value
			case "hours":
				hours = value
			}
		}
	}

	return months, days, hours, nil
}

func CalculateUnrealizedPL(vwap Vwap, vol Volume, tick Tick) float64 {
	if vol > 0 {
		return (tick.Price - float64(vwap)) * float64(vol)
	} else if vol < 0 {
		return (float64(vwap) - tick.Price) * math.Abs(float64(vol))
	} else {
		return 0
	}
}

func PriceLevelProfitLossAboveZeroConstraint(priceLevel *PriceLevel, _ *ExitCondition, params map[string]interface{}) (bool, error) {
	tick := params["tick"].(Tick)
	stats, err := priceLevel.Trades.GetTradeStats(tick)
	if err != nil {
		return false, fmt.Errorf("PriceLevelProfitLossAboveZeroConstraint: failed to get trade stats: %w", err)
	}

	pl := stats.RealizedPL + stats.FloatingPL

	return pl > 0, nil
}

func ReverseCandlesDTO(candles []*CandleDTO) []*CandleDTO {
	for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
		candles[i], candles[j] = candles[j], candles[i]
	}
	return candles
}

func ImportAndSortCandles(inDir string, timeframe time.Duration) (TradingViewCandles, error) {
	f, err := os.Open(inDir)
	if err != nil {
		return TradingViewCandles{}, fmt.Errorf("error opening file: %v", err)
	}

	defer f.Close()

	r := csv.NewReader(f)

	var candlesDTO TradingViewCandlesDTO

	gocsv.UnmarshalCSV(r, &candlesDTO)

	candles := candlesDTO.ToModel()

	candlesSorted := SortCandles(candles, timeframe)

	if err := candlesSorted.Validate(); err != nil {
		return nil, fmt.Errorf("error validating candles: %v", err)
	}

	return candlesSorted, nil
}

func SortCandles(candles TradingViewCandles, timeFrame time.Duration) TradingViewCandles {
	xValues := map[time.Time]*TradingViewCandle{}

	// remove duplicates
	for _, candle := range candles {
		xValues[candle.Timestamp] = candle
	}

	var candlesNoDuplicates []*TradingViewCandle
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

func IsMarkedClosed(timestamp time.Time) bool {
	// Load the New York time zone
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatalf("Error loading location: %v", err)
	}

	// Convert the timestamp to the New York time zone
	nyTime := timestamp.In(loc)

	if nyTime.Weekday() == time.Saturday || nyTime.Weekday() == time.Sunday {
		return true
	}

	// yes if > 4pm EST and < 9:30am EST
	if nyTime.Hour() > 16 || (nyTime.Hour() == 16 && nyTime.Minute() >= 0) || nyTime.Hour() < 9 || (nyTime.Hour() == 9 && nyTime.Minute() < 30) {
		return true
	}

	return false
}
