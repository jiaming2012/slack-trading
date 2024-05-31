package eventmodels

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

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

func ConvertToMarketClose(time.Time) (time.Time, error) {
	// market close is 4:00 PM EST
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to load location: %w", err)
	}

	return time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 16, 0, 0, 0, loc), nil
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
