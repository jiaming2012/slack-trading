package eventmodels

import (
	"fmt"
	"time"
)

type PolygonTimespan struct {
	Multiplier int
	Unit       PolygonTimespanUnit
}

func (p PolygonTimespan) ToDuration() time.Duration {
	switch p.Unit {
	case PolygonTimespanUnitSecond:
		return time.Duration(p.Multiplier) * time.Second
	case PolygonTimespanUnitMinute:
		return time.Duration(p.Multiplier) * time.Minute
	case PolygonTimespanUnitHour:
		return time.Duration(p.Multiplier) * time.Hour
	case PolygonTimespanUnitDay:
		return time.Duration(p.Multiplier) * 24 * time.Hour
	case PolygonTimespanUnitWeek:
		return time.Duration(p.Multiplier) * 7 * 24 * time.Hour
	case PolygonTimespanUnitMonth:
		return time.Duration(p.Multiplier) * 30 * 24 * time.Hour
	case PolygonTimespanUnitQuarter:
		return time.Duration(p.Multiplier) * 90 * 24 * time.Hour
	case PolygonTimespanUnitYear:
		return time.Duration(p.Multiplier) * 365 * 24 * time.Hour
	default:
		panic("invalid timespan unit")
	}
}

func NewPolygonTimespanRequest(period time.Duration) (PolygonTimespan, error) {
	switch period {
	case 1 * time.Minute:
		return PolygonTimespan{
			Multiplier: 1,
			Unit:       "minute",
		}, nil
	case 5 * time.Minute:
		return PolygonTimespan{
			Multiplier: 5,
			Unit:       "minute",
		}, nil
	case 15 * time.Minute:
		return PolygonTimespan{
			Multiplier: 15,
			Unit:       "minute",
		}, nil
	case 30 * time.Minute:
		return PolygonTimespan{
			Multiplier: 30,
			Unit:       "minute",
		}, nil
	case 1 * time.Hour:
		return PolygonTimespan{
			Multiplier: 1,
			Unit:       "hour",
		}, nil
	case 4 * time.Hour:
		return PolygonTimespan{
			Multiplier: 4,
			Unit:       "hour",
		}, nil
	case 24 * time.Hour:
		return PolygonTimespan{
			Multiplier: 1,
			Unit:       "day",
		}, nil
	case 7 * 24 * time.Hour:
		return PolygonTimespan{
			Multiplier: 1,
			Unit:       "week",
		}, nil
	default:
		return PolygonTimespan{}, fmt.Errorf("unsupported PolygonTimespanRequest conversion: %v", period)
	}
}
