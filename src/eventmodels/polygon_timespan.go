package eventmodels

import "time"

type PolygonTimespan struct {
	Multiplier int
	Unit       PolygonTimespanUnit
}

func (p PolygonTimespan) ToDuration() time.Duration {
	switch p.Unit {
	case Second:
		return time.Duration(p.Multiplier) * time.Second
	case Minute:
		return time.Duration(p.Multiplier) * time.Minute
	case Hour:
		return time.Duration(p.Multiplier) * time.Hour
	case Day:
		return time.Duration(p.Multiplier) * 24 * time.Hour
	case Week:
		return time.Duration(p.Multiplier) * 7 * 24 * time.Hour
	case Month:
		return time.Duration(p.Multiplier) * 30 * 24 * time.Hour
	case Quarter:
		return time.Duration(p.Multiplier) * 90 * 24 * time.Hour
	case Year:
		return time.Duration(p.Multiplier) * 365 * 24 * time.Hour
	default:
		panic("invalid timespan unit")
	}
}
