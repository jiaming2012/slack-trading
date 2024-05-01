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
	Header      SignalRequestHeader `json:"header"`
	Name        string              `json:"name"`
	LastUpdated time.Time           `json:"lastUpdated"`
}

func (r *CreateSignalRequestEvent) ConvertToTracker() (*Tracker, error) {

	symbol := StockSymbol(r.Header.Symbol)
	if symbol == "" {
		return nil, fmt.Errorf("CreateSignalRequestEvent.ConvertToTracker: symbol was not set")
	}

	return NewSignalTracker(symbol, r.LastUpdated, r.Name, r.GetMetaData().RequestID), nil
}

func (r *CreateSignalRequestEvent) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName: AccountsStream,
		EventName:  CreateSignalRequestEventName,
	}
}

func NewSignalRequest(requestID uuid.UUID, name string) *CreateSignalRequestEvent {
	request := &CreateSignalRequestEvent{Name: name}
	request.SetMetaData(&MetaData{RequestID: requestID})

	return request
}

func (r *CreateSignalRequestEvent) String() string {
	return fmt.Sprintf("SignalRequest: %v, source=%v", r.Name, r.Header.Source)
}

func (r *CreateSignalRequestEvent) GetSource() SignalRequestSource {
	return r.Header.Source
}

func (r *CreateSignalRequestEvent) Validate(req *http.Request) error {
	if r.Name == "" {
		return fmt.Errorf("SignalRequest.Validate: name was not set")
	}

	return nil
}

func (r *CreateSignalRequestEvent) ParseHTTPRequest(req *http.Request) error {
	if err := json.NewDecoder(req.Body).Decode(r); err != nil {
		return fmt.Errorf("SignalRequest.ParseHTTPRequest: failed to unmarshal request body: %w", err)
	}

	if r.Name == "" {
		return fmt.Errorf("SignalRequest.ParseHTTPRequest: account name was not found")
	}

	return nil
}
