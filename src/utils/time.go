package utils

import "time"

func DeriveNextFriday(now time.Time) time.Time {
	// find the next friday
	for {
		if now.Weekday() == time.Friday {
			break
		}

		now = now.AddDate(0, 0, 1)
	}

	return now
}
