package models

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// todo: deprecated for event models
type NewSignalRequestEvent struct {
	RequestID   uuid.UUID     `json:"requestID"`
	Name        string        `json:"name"`
	Source      RequestSource `json:"source"`
	LastUpdated time.Time     `json:"lastUpdated"`
}

func NewSignalRequest(requestID uuid.UUID, name string) *NewSignalRequestEvent {
	return &NewSignalRequestEvent{RequestID: requestID, Name: name}
}

func (r *NewSignalRequestEvent) String() string {
	return fmt.Sprintf("SignalRequest: %v, source=%v", r.Name, r.Source)
}

func (r *NewSignalRequestEvent) GetRequestID() uuid.UUID {
	return r.RequestID
}

func (r *NewSignalRequestEvent) SetRequestID(id uuid.UUID) {
	r.RequestID = id
	r.LastUpdated = time.Now().UTC()
}

func (r *NewSignalRequestEvent) GetSource() RequestSource {
	return r.Source
}

func (r *NewSignalRequestEvent) Validate(req *http.Request) error {
	if r.Name == "" {
		return fmt.Errorf("SignalRequest.Validate: name was not set")
	}

	return nil
}

func (r *NewSignalRequestEvent) ParseHTTPRequest(req *http.Request) error {
	var values map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&values); err != nil {
		return fmt.Errorf("SignalRequest.ParseHTTPRequest: failed to decode json: %w", err)
	}

	if payload, found := values["payload"]; found {
		if val, ok := payload.(string); ok {
			r.Name = val
		}
	}

	if r.Name == "" {
		return fmt.Errorf("GetStatsRequest.Validate: account name was not found")
	}

	return nil
}
