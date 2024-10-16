package models

import (
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type Clock struct {
	CurrentTime time.Time
	EndTime     time.Time
	Calendar    map[string]*eventmodels.Calendar
}

func (c *Clock) Add(timeToAdd time.Duration) {
	c.CurrentTime = c.CurrentTime.Add(timeToAdd)

	if c.Calendar != nil {
		today, ok := c.Calendar[c.CurrentTime.Format("2006-01-02")]
		if ok {
			if !today.IsBetweenMarketHours(c.CurrentTime) {
				c.advanceToNextMarketOpen()
				return
			}
		}
	}
}

func (c *Clock) IsExpired() bool {
	return c.CurrentTime.Equal(c.EndTime) || c.CurrentTime.After(c.EndTime)
}

func (c *Clock) advanceToNextMarketOpen() {
	// Load the New York time zone
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatalf("Error loading location: %v", err)
	}

	for {
		if c.IsExpired() {
			return
		}

		c.CurrentTime = c.CurrentTime.Add(24 * time.Hour)

		calendar, ok := c.Calendar[c.CurrentTime.Format("2006-01-02")]
		if ok {
			c.CurrentTime = calendar.MarketOpen.In(location)
			return
		}
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
