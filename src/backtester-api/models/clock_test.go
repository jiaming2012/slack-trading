package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

// MockFetchCalendarMap returns the mock calendar data
func MockFetchCalendarMap(start, end eventmodels.PolygonDate) (map[string]*eventmodels.Calendar, error) {
	mockData := `{
        "2021-01-12": {"Date": "2021-01-12", "MarketOpen": "2021-01-12T14:30:00Z", "MarketClose": "2021-01-12T21:00:00Z"},
        "2021-01-13": {"Date": "2021-01-13", "MarketOpen": "2021-01-13T14:30:00Z", "MarketClose": "2021-01-13T21:00:00Z"},
        "2021-01-14": {"Date": "2021-01-14", "MarketOpen": "2021-01-14T14:30:00Z", "MarketClose": "2021-01-14T21:00:00Z"},
        "2021-01-15": {"Date": "2021-01-15", "MarketOpen": "2021-01-15T14:30:00Z", "MarketClose": "2021-01-15T21:00:00Z"}
    }`

	var calendar map[string]*eventmodels.Calendar
	err := json.Unmarshal([]byte(mockData), &calendar)
	if err != nil {
		return nil, err
	}

	return calendar, nil
}

func TestCalendar(t *testing.T) {
	t.Run("starts at market open", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 12, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 17, 0, 0, 0, 0, time.UTC)

		calendar, err := MockFetchCalendarMap(eventmodels.PolygonDate{
			Year:  startTime.Year(),
			Month: int(startTime.Month()),
			Day:   startTime.Day(),
		}, eventmodels.PolygonDate{
			Year:  endTime.Year(),
			Month: int(endTime.Month()),
			Day:   endTime.Day(),
		})

		assert.NoError(t, err)

		cJSON, err := json.Marshal(calendar)

		print(string(cJSON))

		assert.NoError(t, err)

		clock := NewClock(startTime, endTime, calendar)

		loc, err := time.LoadLocation("America/New_York")
		assert.NoError(t, err)
		nextMarketOpen := time.Date(2021, time.January, 12, 9, 30, 0, 0, loc)

		assert.Equal(t, nextMarketOpen, clock.CurrentTime)
	})

	t.Run("advances to next market open", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 12, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 17, 0, 0, 0, 0, time.UTC)

		calendar, err := MockFetchCalendarMap(eventmodels.PolygonDate{
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

		assert.Equal(t, nextMarketOpen, clock.CurrentTime)

		// advance to market close
		clock.Add(6 * time.Hour)
		clock.Add(29 * time.Minute)

		assert.Equal(t, time.Date(2021, time.January, 12, 15, 59, 0, 0, loc), clock.CurrentTime)

		// expect: next tick should be next market open
		clock.Add(1 * time.Minute)

		assert.Equal(t, time.Date(2021, time.January, 13, 9, 30, 0, 0, loc), clock.CurrentTime)
	})
}
