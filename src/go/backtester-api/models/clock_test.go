package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/utils"
)

// MockFetchCalendarMap returns the mock calendar data
func MockFetchCalendarMap(start, end models.PolygonDate) (map[string]*models.Calendar, error) {
	mockData := `{
        "2021-01-12": {"Date": "2021-01-12", "MarketOpen": "2021-01-12T14:30:00Z", "MarketClose": "2021-01-12T21:00:00Z"},
        "2021-01-13": {"Date": "2021-01-13", "MarketOpen": "2021-01-13T14:30:00Z", "MarketClose": "2021-01-13T21:00:00Z"},
        "2021-01-14": {"Date": "2021-01-14", "MarketOpen": "2021-01-14T14:30:00Z", "MarketClose": "2021-01-14T21:00:00Z"},
        "2021-01-15": {"Date": "2021-01-15", "MarketOpen": "2021-01-15T14:30:00Z", "MarketClose": "2021-01-15T21:00:00Z"}
    }`

	var calendar map[string]*models.Calendar
	err := json.Unmarshal([]byte(mockData), &calendar)
	if err != nil {
		return nil, err
	}

	return calendar, nil
}

func TestCalendar(t *testing.T) {
	projectDir, err := utils.GetEnv("TRADING_PROJECT_DIR")
	require.NoError(t, err)

	err = utils.InitEnvironmentVariables(projectDir, "test")
	require.NoError(t, err)

	t.Run("starts at market open", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 12, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 17, 0, 0, 0, 0, time.UTC)

		calendar, err := MockFetchCalendarMap(models.PolygonDate{
			Year:  startTime.Year(),
			Month: int(startTime.Month()),
			Day:   startTime.Day(),
		}, models.PolygonDate{
			Year:  endTime.Year(),
			Month: int(endTime.Month()),
			Day:   endTime.Day(),
		})

		require.NoError(t, err)

		cJSON, err := json.Marshal(calendar)

		print(string(cJSON))

		require.NoError(t, err)

		clock := NewClock(startTime, endTime, calendar)

		loc, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		nextMarketOpen := time.Date(2021, time.January, 12, 9, 30, 0, 0, loc)

		require.Equal(t, nextMarketOpen, clock.CurrentTime)
	})

	createPlayground := func(symbol models.Instrument, clock *Clock, feed []*models.PolygonAggregateBarV2) (*Playground, error) {
		period := time.Minute
		env := PlaygroundEnvironmentSimulator
		source := models.CandleRepositorySource{
			Type: "test",
		}

		repo, err := NewCandleRepository(symbol, period, feed, []string{}, nil, 0, source)
		require.NoError(t, err)

		balance := 1000.0
		playground, err := NewPlayground(PlaygroundConfig{
			Balance: balance,
			Clock:   clock,
			Env:     env,
			Now:     clock.CurrentTime,
			Feeds:   []*CandleRepository{repo},
		})
		return playground, err
	}

	t.Run("orders placed outside of market hours are filled at next open", func(t *testing.T) {
		symbol := models.StockSymbol("AAPL")

		startTime := time.Date(2021, time.January, 12, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 14, 0, 0, 0, 0, time.UTC)

		t1 := startTime.Add(time.Minute)
		t2 := startTime.Add(2 * time.Minute)

		marketOpenTime := time.Date(2021, time.January, 12, 14, 30, 0, 0, time.UTC)
		s1 := marketOpenTime.Add(1 * time.Minute)

		marketCloseTime := time.Date(2021, time.January, 12, 21, 0, 0, 0, time.UTC)
		u1 := marketCloseTime.Add(1 * time.Minute)

		feed := []*models.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     10.0,
			},
			{
				Timestamp: t1,
				Close:     20.0,
			},
			{
				Timestamp: t2,
				Close:     30.0,
			},
			{
				Timestamp: marketOpenTime,
				Close:     40.0,
			},
			{
				Timestamp: s1,
				Close:     50.0,
			},
			{
				Timestamp: marketCloseTime,
				Close:     60.0,
			},
			{
				Timestamp: u1,
				Close:     70.0,
			},
		}

		calendar, err := MockFetchCalendarMap(models.PolygonDate{
			Year:  startTime.Year(),
			Month: int(startTime.Month()),
			Day:   startTime.Day(),
		}, models.PolygonDate{
			Year:  endTime.Year(),
			Month: int(endTime.Month()),
			Day:   endTime.Day(),
		})

		require.NoError(t, err)

		clock := NewClock(startTime, endTime, calendar)

		playground, err := createPlayground(symbol, clock, feed)
		require.NoError(t, err)

		// place order before market open
		order1, err := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, LiveAccountTypeMock, startTime, string(symbol), TradierOrderSideBuy, 1, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
		require.NoError(t, err)

		changes, err := playground.PlaceOrder(order1)
		require.NoError(t, err)

		require.Len(t, changes, 1)
		err = changes[0].Commit(nil)
		require.NoError(t, err)

		require.Equal(t, startTime, order1.Timestamp)

		// tick
		_, err = playground.Tick(time.Minute, false, nil)
		require.NoError(t, err)

		// expect: order is filled at market open
		require.Equal(t, 1, len(playground.account.Orders))

		require.Equal(t, 1, len(order1.Trades))

		require.Equal(t, marketOpenTime.UTC(), order1.Trades[0].Timestamp.UTC())
	})

	t.Run("advances to next market open", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 12, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 17, 0, 0, 0, 0, time.UTC)

		calendar, err := MockFetchCalendarMap(models.PolygonDate{
			Year:  startTime.Year(),
			Month: int(startTime.Month()),
			Day:   startTime.Day(),
		}, models.PolygonDate{
			Year:  endTime.Year(),
			Month: int(endTime.Month()),
			Day:   endTime.Day(),
		})

		require.NoError(t, err)

		clock := NewClock(startTime, endTime, calendar)

		loc, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		nextMarketOpen := time.Date(2021, time.January, 12, 9, 30, 0, 0, loc)

		require.Equal(t, nextMarketOpen, clock.CurrentTime)

		// advance to market close
		clock.Add(6 * time.Hour)
		clock.Add(29 * time.Minute)

		require.Equal(t, time.Date(2021, time.January, 12, 15, 59, 0, 0, loc), clock.CurrentTime)

		// expect: next tick should be next market open
		clock.Add(1 * time.Minute)

		require.Equal(t, time.Date(2021, time.January, 13, 9, 30, 0, 0, loc), clock.CurrentTime)
	})
}
