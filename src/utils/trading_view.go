package utils

import (
	"log"
	"time"
)

func IsMarkedClosed(timestamp time.Time) bool {
	// Load the New York time zone
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatalf("Error loading location: %v", err)
	}

	// Convert the timestamp to the New York time zone
	nyTime := timestamp.In(loc)

	if nyTime.Weekday() == time.Saturday || nyTime.Weekday() == time.Sunday {
		return true
	}

	// yes if > 4pm EST and < 9:30am EST
	if nyTime.Hour() > 16 || (nyTime.Hour() == 16 && nyTime.Minute() >= 0) || nyTime.Hour() < 9 || (nyTime.Hour() == 9 && nyTime.Minute() < 30) {
		return true
	}

	return false
}
