package eventservices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

var cachedPayload *eventmodels.MarketCalendar

func IsMarketOpen(calendar *eventmodels.MarketCalendar, now time.Time) (bool, error) {
	dateStr := now.Format("2006-01-02")
	timeStr := now.Format("15:04")

	for _, day := range calendar.Calendar.Days.Day {
		if day.Date == dateStr {
			if day.Status == "open" {
				start, err := time.Parse("15:04", day.Open.Start)
				if err != nil {
					return false, err
				}
				end, err := time.Parse("15:04", day.Open.End)
				if err != nil {
					return false, err
				}
				currentTime, err := time.Parse("15:04", timeStr)
				if err != nil {
					return false, err
				}

				if currentTime.After(start) && currentTime.Before(end) {
					return true, nil
				}
			}
			break
		}
	}

	return false, nil
}

func FetchMarketCalendar(url, bearerToken string, now time.Time) (*eventmodels.MarketCalendar, error) {
	currentMonth := now.Format("2006-01")
	currentMonthInt, err := strconv.Atoi(currentMonth[5:])
	if err != nil {
		return nil, fmt.Errorf("FetchMarketCalendar: failed to parse current month: %w", err)
	}

	if cachedPayload != nil && cachedPayload.Calendar.Month == currentMonthInt {
		return cachedPayload, nil
	}

	log.Debugf("Cache invalid. Fetching market calendar for %v", currentMonth)

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("FetchMarketCalendar: failed to create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FetchMarketCalendar: failed to fetch market calendar: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FetchMarketCalendar: failed to fetch market calendar, http code %v", res.Status)
	}

	var dto eventmodels.MarketCalendar
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("FetchMarketCalendar: failed to decode json: %w", err)
	}

	cachedPayload = &dto

	return &dto, nil
}
