package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type CreateAccountRequestEventV1 struct {
	BaseRequestEvent
	Name         string       `json:"name"`
	Balance      float64      `json:"balance"`
	DatafeedName DatafeedName `json:"datafeedName"`
}

func (e *CreateAccountRequestEventV1) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName:    AccountsStream,
		EventName:     CreateAccountRequestEventName,
		SchemaVersion: 1,
	}
}

func (e *CreateAccountRequestEventV1) Validate(r *http.Request) error {
	if e.Name == "" {
		return fmt.Errorf("CreateAccountRequestEvent.Validate: name was not set")
	}

	if e.Balance <= 0 {
		return fmt.Errorf("CreateAccountRequestEvent.Validate: balance was not set")
	}

	if e.DatafeedName == "" {
		return fmt.Errorf("CreateAccountRequestEvent.Validate: datafeedName was not set")
	}

	return nil
}

func (e *CreateAccountRequestEventV1) ParseHTTPRequest(r *http.Request) error {
	var values map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&values); err != nil {
		return fmt.Errorf("PostAccountsRequestEvent.ParseHTTPRequest: failed to decode json: %w", err)
	}

	if payload, found := values["name"]; found {
		if val, ok := payload.(string); ok {
			e.Name = val
		}
	} else {
		return fmt.Errorf("PostAccountsRequestEvent.ParseHTTPRequest: name was not found")
	}

	if payload, found := values["balance"]; found {
		if val, ok := payload.(float64); ok {
			e.Balance = val
		}
	} else {
		return fmt.Errorf("PostAccountsRequestEvent.ParseHTTPRequest: balance was not found")
	}

	if payload, found := values["datafeedName"]; found {
		if val, ok := payload.(string); ok {
			e.DatafeedName = DatafeedName(val)
		}
	} else {
		return fmt.Errorf("PostAccountsRequestEvent.ParseHTTPRequest: datafeedName was not found")
	}

	return nil
}
