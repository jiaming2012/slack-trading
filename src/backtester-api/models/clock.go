package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type Clock struct {
	CurrentTime time.Time
	EndTime     time.Time
	Calendar    map[string]*eventmodels.Calendar
}

func (c *Clock) Add(timeToAdd time.Duration) {
	if c.Calendar != nil {
		today, ok := c.Calendar[c.CurrentTime.Format("2006-01-02")]
		if ok {
			if !today.IsBetweenMarketHours(c.CurrentTime) {
				c.advanceToNextMarketOpen()
				return
			}
		}
	}

	c.CurrentTime = c.CurrentTime.Add(timeToAdd)
}

func (c *Clock) IsExpired() bool {
	return c.CurrentTime.Equal(c.EndTime) || c.CurrentTime.After(c.EndTime)
}

func (c *Clock) advanceToNextMarketOpen() {
	for {
		if c.IsExpired() {
			return
		}

		calendar, ok := c.Calendar[c.CurrentTime.Format("2006-01-02")]
		if ok {
			if c.CurrentTime.Equal(calendar.MarketOpen) || c.CurrentTime.After(calendar.MarketOpen) && c.CurrentTime.Before(calendar.MarketClose) {
				return
			}
		}
		c.CurrentTime = c.CurrentTime.Add(30 * time.Minute)
	}
}

func NewClock(startTime time.Time, endTime time.Time, calendar map[string]*eventmodels.Calendar) *Clock {
	clock := &Clock{
		CurrentTime: startTime,
		EndTime:     endTime,
		Calendar:    calendar,
	}

	if calendar != nil {
		clock.advanceToNextMarketOpen()
	}

	return clock
}
