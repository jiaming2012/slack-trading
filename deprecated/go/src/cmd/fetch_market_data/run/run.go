package run

import (
	"fmt"
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func GetOrderDateRange(order *eventmodels.TradierOrder) (time.Time, time.Time) {
	panic("not implemented")
}

func GetCandleAtDate(at time.Time, candles []*eventmodels.CandleDTO) (*eventmodels.CandleDTO, error) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("GetCandleAtDate: failed to load location: %w", err)
	}

	// Convert the time to EST
	createdAtEst := at.In(loc)
	target := createdAtEst.Format("2006-01-02 15:04:00")
	for _, candle := range candles {
		if candle.Date == target {
			return candle, nil
		}
	}

	return nil, fmt.Errorf("GetCandleAtDate: no candle found at %v", at.Format("2006-01-02 15:04"))
}

func TransformDateTime(candles []*eventmodels.CandleDTO) error {
	for _, candle := range candles {
		// Parse the input string assuming it's in a specific format.
		parsedTime, err := time.Parse("2006-01-02 15:04:05", candle.Date)
		if err != nil {
			return fmt.Errorf("TransformDateTime: failed to parse time: %w", err)
		}

		candle.Date = parsedTime.Format("2006-01-02 15:04")
	}

	return nil
}
