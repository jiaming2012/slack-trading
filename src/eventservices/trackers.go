package eventservices

import (
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func GetActiveFxTrackers(trackers []*eventmodels.TrackerV3) map[eventmodels.EventStreamID]*eventmodels.TrackerV3 {
	activeTrackersMap := make(map[eventmodels.EventStreamID]*eventmodels.TrackerV3)

	for _, tracker := range trackers {
		if tracker.Type == eventmodels.TrackerTypeStartFx {
			id := tracker.GetMetaData().GetEventStreamID()
			activeTrackersMap[id] = tracker
		}
	}

	for _, tracker := range trackers {
		if tracker.Type == eventmodels.TrackerTypeStop {
			delete(activeTrackersMap, tracker.StopTracker.TrackerStartID)
		}
	}

	return activeTrackersMap
}

func GetActiveStockAndOptionTrackers(trackers map[eventmodels.EventStreamID]*eventmodels.TrackerV3) map[eventmodels.EventStreamID]*eventmodels.TrackerV3 {
	activeTrackers := make(map[eventmodels.EventStreamID]*eventmodels.TrackerV3)

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
