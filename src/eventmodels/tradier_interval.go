package eventmodels

import "fmt"

type TradierInterval string

const (
	TradierInterval1Min  TradierInterval = "1min"
	TradierInterval5Min  TradierInterval = "5min"
	TradierInterval15Min TradierInterval = "15min"
)

func NewTradierInterval(interval PolygonTimespan) (TradierInterval, error) {
	if interval.Unit != PolygonTimespanUnitMinute {
		return "", fmt.Errorf("invalid timespan unit")
	}

	switch interval.Multiplier {
	case 1:
		return TradierInterval1Min, nil
	case 5:
		return TradierInterval5Min, nil
	case 15:
		return TradierInterval15Min, nil
	default:
		return "", fmt.Errorf("invalid timespan multiplier")
	}
}
