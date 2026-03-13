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

func maxVal(vals []int) int {
	max := vals[0]
	for _, v := range vals {
		if v > max {
			max = v
		}
	}

	return max
}

func DeriveNextExpiration(now time.Time, expirationInDays []int) time.Time {
	// find the min expiration
	minDays := maxVal(expirationInDays)

	// start from minDays
	now = now.AddDate(0, 0, minDays)

	// find the next expiration
	for {
		if now.Weekday() != time.Saturday && now.Weekday() != time.Sunday {
			break
		}

		now = now.AddDate(0, 0, 1)
	}

	return now
}