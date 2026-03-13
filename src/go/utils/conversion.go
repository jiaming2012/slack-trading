package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// AtoiSlice converts a comma-separated string to an int slice
func AtoiSlice(s string) ([]int, error) {
	strVals := strings.Split(s, ",")
	intVals := make([]int, len(strVals))
	for i, strVal := range strVals {
		strVal = strings.TrimSpace(strVal)
		intVal, err := strconv.Atoi(strVal)
		if err != nil {
			return nil, fmt.Errorf("failed to convert '%s' to int: %v", strVal, err)
		}
		intVals[i] = intVal
	}

	return intVals, nil
}

func UnixMillisToTime(timestampMs int64) time.Time {
	// Convert milliseconds to nanoseconds and create a time.Time object
	return time.Unix(0, timestampMs*int64(time.Millisecond)).UTC()
}

func ParseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("failed to convert '%s' to bool", s)
	}
}
