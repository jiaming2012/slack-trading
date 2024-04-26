package eventmodels

import "time"

type TrackerStop struct {
	ID             TrackerID `json:"id"`
	TrackerStartID TrackerID `json:"trackerStartID"`
	Timestamp      time.Time `json:"timestamp"`
	Reason         string    `json:"reason"`
}
