package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// todo: deprecated for event models
type CreateSignalRequestEventV1 struct {
	BaseRequestEvent
	Header      SignalRequestHeader `json:"header"`
	Name        string              `json:"name"`
	LastUpdated time.Time           `json:"lastUpdated"`
}

func (r *CreateSignalRequestEventV1) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName:    AccountsStream,
		EventName:     CreateSignalRequestEventName,
		SchemaVersion: 1,
	}
}

func (r *CreateSignalRequestEventV1) ConvertToTracker(now time.Time) (*TrackerV2, error) {
	symbol := StockSymbol(r.Header.Symbol)
	if symbol == "" {
		return nil, fmt.Errorf("CreateSignalRequestEvent.ConvertToTracker: symbol was not set")
	}

	return NewSignalTrackerV2(r.Name, r.Header, now, r.GetMetaData().RequestID), nil
}

func NewSignalRequest(requestID uuid.UUID, name string) *CreateSignalRequestEventV1 {
	request := &CreateSignalRequestEventV1{Name: name}
	request.SetMetaData(&MetaData{RequestID: requestID})

	return request
}

func (r *CreateSignalRequestEventV1) String() string {
	return fmt.Sprintf("SignalRequest: %v, source=%v", r.Name, r.Header.Source)
}

func (r *CreateSignalRequestEventV1) GetSource() SignalSource {
	return r.Header.Source
}

func (r *CreateSignalRequestEventV1) Validate(req *http.Request) error {
	if r.Name == "" {
		return fmt.Errorf("SignalRequest.Validate: name was not set")
	}

	return nil
}

func (r *CreateSignalRequestEventV1) ParseHTTPRequest(req *http.Request) error {
	if err := json.NewDecoder(req.Body).Decode(r); err != nil {
		return fmt.Errorf("SignalRequest.ParseHTTPRequest: failed to unmarshal request body: %w", err)
	}

	if r.Name == "" {
		return fmt.Errorf("SignalRequest.ParseHTTPRequest: account name was not found")
	}

	return nil
}
