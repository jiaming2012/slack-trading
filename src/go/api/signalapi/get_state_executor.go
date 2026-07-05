package signalapi

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/jiaming2012/slack-trading/src/go/workers"
	"github.com/jiaming2012/slack-trading/src/go/models"
)

type GetStateExecutor struct {
	tracker *workers.TrackerConsumerV3
}

func (s *GetStateExecutor) Serve(r *http.Request, request models.ApiRequest3, resultCh chan interface{}, errCh chan error) {
	state, unlock := s.tracker.GetState()
	defer unlock()

	// Create a deep copy of state
	stateBytes, err := json.Marshal(state)
	if err != nil {
		log.Printf("Failed to marshal state: %v", err)
		errCh <- err
		return
	}

	stateCopy := make(map[string]interface{})
	err = json.Unmarshal(stateBytes, &stateCopy)
	if err != nil {
		log.Printf("Failed to unmarshal state: %v", err)
		errCh <- err
		return
	}

	// Send the copy of state to resultCh
	resultCh <- stateCopy
}

func NewGetStateExecutor(tracker *workers.TrackerConsumerV3) *GetStateExecutor {
	return &GetStateExecutor{
		tracker: tracker,
	}
}
