package eventmodels

import "time"

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
