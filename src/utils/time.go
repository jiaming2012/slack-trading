package utils

import "time"

func GetMinTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}

	return b
}
