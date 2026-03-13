package helpers

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func GetHeadersFromStruct(i interface{}) []string {
	t := reflect.TypeOf(i)
	headers := make([]string, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		headers[i] = t.Field(i).Name
	}
	return headers
}

func GetDurationFromStreamName(streamName string) (time.Duration, error) {
	if streamName[0:7] != "candles" {
		return 0, fmt.Errorf("invalid stream name: expected stream name to start with 'candles'")
	}

	components := strings.Split(streamName, "-")
	if len(components) != 3 {
		return 0, fmt.Errorf("invalid stream name: expected stream name to have 3 components ['candles', 'underlying_symbol', 'duration], found %v components", components)
	}

	duration := components[2]

	// check if duration has D or W
	if strings.Contains(duration, "D") {
		// check number of days
		daysStr := strings.Split(duration, "D")
		if len(daysStr) != 2 {
			return 0, fmt.Errorf("invalid duration: expected duration to have 2 components ['number', 'D'], found %v components", daysStr)
		}

		days, err := strconv.Atoi(daysStr[0])
		if err != nil {
			return 0, fmt.Errorf("invalid duration: expected duration to represent number of days, found %v", daysStr[0])
		}

		return time.Duration(days) * 24 * time.Hour, nil
	}

	if strings.Contains(duration, "W") {
		// check number of weeks
		weeksStr := strings.Split(duration, "W")
		if len(weeksStr) != 2 {
			return 0, fmt.Errorf("invalid duration: expected duration to have 2 components ['number', 'W'], found %v components", weeksStr)
		}

		weeks, err := strconv.Atoi(weeksStr[0])
		if err != nil {
			return 0, fmt.Errorf("invalid duration: expected duration to represent number of weeks, found %v", weeksStr[0])
		}

		return time.Duration(weeks) * 7 * 24 * time.Hour, nil
	}

	mins, err := strconv.Atoi(duration)
	if err != nil {
		return 0, fmt.Errorf("invalid duration: expected duration to be represent number of minutes, found %v", duration)
	}

	return time.Duration(mins) * time.Minute, nil
}
