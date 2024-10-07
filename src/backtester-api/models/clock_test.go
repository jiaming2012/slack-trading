package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jiaming2012/slack-trading/src/backtester-api/services"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestCalendar(t *testing.T) {
	t.Run("starts at market open", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 12, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 17, 0, 0, 0, 0, time.UTC)

		calendar, err := services.FetchCalendarMap(eventmodels.PolygonDate{
			Year:  startTime.Year(),
			Month: int(startTime.Month()),
			Day:   startTime.Day(),
		}, eventmodels.PolygonDate{
			Year:  endTime.Year(),
			Month: int(endTime.Month()),
			Day:   endTime.Day(),
		})

		assert.NoError(t, err)

		clock := NewClock(startTime, endTime, calendar)

		loc, err := time.LoadLocation("America/New_York")
		assert.NoError(t, err)
		nextMarketOpen := time.Date(2021, time.January, 12, 9, 30, 0, 0, loc)

		assert.Equal(t, nextMarketOpen.UTC(), clock.CurrentTime)
	})

	t.Run("advances to next market open", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 12, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 17, 0, 0, 0, 0, time.UTC)

		calendar, err := services.FetchCalendarMap(eventmodels.PolygonDate{
			Year:  startTime.Year(),
			Month: int(startTime.Month()),
			Day:   startTime.Day(),
		}, eventmodels.PolygonDate{
			Year:  endTime.Year(),
			Month: int(endTime.Month()),
			Day:   endTime.Day(),
		})

		assert.NoError(t, err)

		clock := NewClock(startTime, endTime, calendar)

		loc, err := time.LoadLocation("America/New_York")
		assert.NoError(t, err)
		nextMarketOpen := time.Date(2021, time.January, 12, 9, 30, 0, 0, loc)

		assert.Equal(t, nextMarketOpen.UTC(), clock.CurrentTime)

		// advance to market close
		clock.Add(6 * time.Hour)
		clock.Add(30 * time.Minute)

		assert.Equal(t, time.Date(2021, time.January, 12, 16, 0, 0, 0, loc).UTC(), clock.CurrentTime)

		// expect: next tick should be next market open
		clock.Add(1 * time.Minute)

		assert.Equal(t, time.Date(2021, time.January, 13, 9, 30, 0, 0, loc).UTC(), clock.CurrentTime)
	})
}
