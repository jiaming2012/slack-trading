package models

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net/http"
)

type SignalRequest struct {
	RequestID uuid.UUID     `json:"requestID"`
	Name      string        `json:"name"`
	Source    RequestSource `json:"source"`
}

func NewSignalRequest(requestID uuid.UUID, name string) *SignalRequest {
	return &SignalRequest{RequestID: requestID, Name: name}
}

func (r *SignalRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}

func (r *SignalRequest) SetRequestID(id uuid.UUID) {
	r.RequestID = id
}

func (r *SignalRequest) GetSource() RequestSource {
	return r.Source
}

func (r *SignalRequest) ParseHTTPRequest(req *http.Request) error {
	if err := json.NewDecoder(req.Body).Decode(r); err != nil {
		return fmt.Errorf("SignalRequest.ParseHTTPRequest: failed to decode json: %w", err)
	}

	if r.Name == "" {
		return fmt.Errorf("GetStatsRequest.Validate: account name was not found")
	}

	return nil
}
