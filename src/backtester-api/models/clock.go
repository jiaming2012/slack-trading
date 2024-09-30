package models

import "time"

type Clock struct {
	CurrentTime time.Time
	EndTime     time.Time
}

func (c *Clock) Add(timeToAdd time.Duration) {
	c.CurrentTime = c.CurrentTime.Add(timeToAdd)
}

func (c *Clock) IsExpired() bool {
	return c.CurrentTime.Equal(c.EndTime) || c.CurrentTime.After(c.EndTime)
}

func NewClock(startTime time.Time, endTime time.Time) *Clock {
	return &Clock{
		CurrentTime: startTime,
		EndTime:     endTime,
	}
}
