package models

import (
	"fmt"
	"time"
)

type CalendarRepository map[string]*Calendar

func (r CalendarRepository) SetCalendar(date string, calendar *Calendar) {
	r[date] = calendar
}

func (r CalendarRepository) IsMarketOpen(ts time.Time) bool {
	calendar, exists := r[ts.Format("2006-01-02")]
	if !exists {
		return false
	}

	return calendar.IsBetweenMarketHours(ts)
}

func (r CalendarRepository) GetNextMarketOpen(ts time.Time) (*Calendar, error) {
	maxAttempts := 365
	attempts := 0
	for {
		ts = ts.Add(24 * time.Hour)
		calendar, exists := r[ts.Format("2006-01-02")]
		if exists {
			if calendar.IsBetweenMarketHours(ts) {
				return calendar, nil
			}
		}

		attempts++
		if attempts >= maxAttempts {
			return nil, fmt.Errorf("GetNextMarketOpen: reached max attempts")
		}
	}
}

func NewCalendarRepository() CalendarRepository {
	return make(CalendarRepository)
}
