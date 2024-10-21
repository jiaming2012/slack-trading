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
	location    *time.Location
}

func (c *Clock) GetNext(currentTime time.Time, timeToAdd time.Duration) time.Time {
	currentTime = currentTime.Add(timeToAdd)

	if c.Calendar != nil {
		today, ok := c.Calendar[currentTime.Format("2006-01-02")]
		if ok {
			if !today.IsBetweenMarketHours(currentTime) {
				currentTime = currentTime.Add(24 * time.Hour)
				c.advanceToNextMarketOpen(&currentTime)
				return currentTime
			}
		} else {
			c.advanceToNextMarketOpen(&currentTime)
		}
	}

	return currentTime
}

func (c *Clock) Add(timeToAdd time.Duration) {
	nextTime := c.GetNext(c.CurrentTime, timeToAdd)
	c.CurrentTime = nextTime
}

func (c *Clock) IsExpired() bool {
	return c.IsTimeExpired(c.CurrentTime)
}

func (c *Clock) IsTimeExpired(timeToCheck time.Time) bool {
	return timeToCheck.Equal(c.EndTime) || timeToCheck.After(c.EndTime)
}

func (c *Clock) advanceToNextMarketOpen(currentTime *time.Time) {
	for {
		calendar, ok := c.Calendar[currentTime.Format("2006-01-02")]
		if ok {
			*currentTime = calendar.MarketOpen.In(c.location)
			return
		}

		if c.IsExpired() {
			return
		}

		*currentTime = currentTime.Add(24 * time.Hour)
	}
}

func NewClock(startTime time.Time, endTime time.Time, calendar map[string]*eventmodels.Calendar) *Clock {
	// Load the New York time zone
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatalf("Error loading location: %v", err)
	}

	clock := &Clock{
		CurrentTime: startTime,
		EndTime:     endTime,
		Calendar:    calendar,
		location:    location,
	}

	if calendar != nil {
		today, ok := calendar[clock.CurrentTime.Format("2006-01-02")]
		if ok {
			if !today.IsBetweenMarketHours(clock.CurrentTime) {
				clock.advanceToNextMarketOpen(&clock.CurrentTime)
			}
		}
	}

	return clock
}
