package eventservices

import (
	"slack-trading/src/eventmodels"
)

func GetActiveTrackers(trackers map[eventmodels.EventStreamID]*eventmodels.TrackerV1) map[eventmodels.EventStreamID]*eventmodels.TrackerV1 {
	activeTrackers := make(map[eventmodels.EventStreamID]*eventmodels.TrackerV1)

	for _, tracker := range trackers {
		if tracker.Type == eventmodels.TrackerTypeStart {
			id := tracker.GetMetaData().GetEventStreamID()
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
