package eventservices

import (
	"github.com/jiaming2012/slack-trading/src/go/models"
)

func GetActiveFxTrackers(trackers []*models.TrackerV3) map[models.EventStreamID]*models.TrackerV3 {
	activeTrackersMap := make(map[models.EventStreamID]*models.TrackerV3)

	for _, tracker := range trackers {
		if tracker.Type == models.TrackerTypeStartFx {
			id := tracker.GetMetaData().GetEventStreamID()
			if tracker.StartFxTracker != nil && tracker.StartFxTracker.Symbol != "" {
				activeTrackersMap[id] = tracker
			}
		}
	}

	for _, tracker := range trackers {
		if tracker.Type == models.TrackerTypeStop {
			delete(activeTrackersMap, tracker.StopTracker.TrackerStartID)
		}
	}

	return activeTrackersMap
}

func GetActiveStockAndOptionTrackers(trackers map[models.EventStreamID]*models.TrackerV3) map[models.EventStreamID]*models.TrackerV3 {
	activeTrackers := make(map[models.EventStreamID]*models.TrackerV3)

	for _, tracker := range trackers {
		if tracker.Type == models.TrackerTypeStart {
			id := tracker.GetMetaData().GetEventStreamID()
			activeTrackers[id] = tracker
		}
	}

	for _, tracker := range trackers {
		if tracker.Type == models.TrackerTypeStop {
			delete(activeTrackers, tracker.StopTracker.TrackerStartID)
		}
	}

	return activeTrackers
}
