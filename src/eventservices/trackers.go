package eventservices

import (
	"slack-trading/src/eventmodels"
)

func GetActiveTrackers(trackers map[eventmodels.EventStreamID]*eventmodels.Tracker) map[eventmodels.EventStreamID]*eventmodels.Tracker {
	activeTrackers := make(map[eventmodels.EventStreamID]*eventmodels.Tracker)

	for _, tracker := range trackers {
		if tracker.Type == eventmodels.TrackerTypeStart {
			id := tracker.GetMetaData().EventStreamID
			activeTrackers[id] = tracker
		}
	}

	for _, tracker := range trackers {
		if tracker.Type == eventmodels.TrackerTypeStop {
			delete(activeTrackers, tracker.StopTracker.TrackerStartID)
		}
	}

	return activeTrackers
}
