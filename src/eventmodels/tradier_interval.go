package eventmodels

import "fmt"

type TradierInterval string

const (
	TradierInterval1Min  TradierInterval = "1min"
	TradierInterval5Min  TradierInterval = "5min"
	TradierInterval15Min TradierInterval = "15min"
)

func (i TradierInterval) ToPolygonInterval() PolygonTimespan {
	switch i {
	case TradierInterval1Min:
		return PolygonTimespan{
			Unit:      PolygonTimespanUnitMinute,
			Multiplier: 1,
		}
	case TradierInterval5Min:
		return PolygonTimespan{
			Unit:      PolygonTimespanUnitMinute,
			Multiplier: 5,
		}
	case TradierInterval15Min:
		return PolygonTimespan{
			Unit:      PolygonTimespanUnitMinute,
			Multiplier: 15,
		}
	default:
		return PolygonTimespan{}
	}
}

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
