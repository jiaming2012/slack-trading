package eventmodels

import "time"

type Calendar struct {
	Date        string
	MarketOpen  time.Time
	MarketClose time.Time
}

func (c *Calendar) IsBetweenMarketHours(t time.Time) bool {
	return (t.Equal(c.MarketOpen) || t.After(c.MarketOpen)) && t.Before(c.MarketClose)
}
