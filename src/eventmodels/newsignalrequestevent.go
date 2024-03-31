package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// todo: deprecated for event models
type CreateSignalRequestEvent struct {
	BaseRequestEvent
	Name        string        `json:"name"`
	Source      RequestSource `json:"source"`
	LastUpdated time.Time     `json:"lastUpdated"`
}

func (r *CreateSignalRequestEvent) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName: AccountsStreamName,
		EventName:  CreateSignalRequestEventName,
	}
}

func NewSignalRequest(requestID uuid.UUID, name string) *CreateSignalRequestEvent {
	request := &CreateSignalRequestEvent{Name: name}
	request.SetMetaData(&MetaData{RequestID: requestID})

	return request
}

func (r *CreateSignalRequestEvent) String() string {
	return fmt.Sprintf("SignalRequest: %v, source=%v", r.Name, r.Source)
}

func (r *CreateSignalRequestEvent) GetSource() RequestSource {
	return r.Source
}

func (r *CreateSignalRequestEvent) Validate(req *http.Request) error {
	if r.Name == "" {
		return fmt.Errorf("SignalRequest.Validate: name was not set")
	}

	return nil
}

func (r *CreateSignalRequestEvent) ParseHTTPRequest(req *http.Request) error {
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
