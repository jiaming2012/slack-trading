package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

type CreateOptionAlertRequestEvent struct {
	BaseRequestEvent2
	CreateOptionAlertDTO
	ID uuid.UUID
}

func (r *CreateOptionAlertRequestEvent) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName: OptionAlertsStreamName,
		EventName:  CreateOptionAlertRequestEventName,
	}
}

func (r *CreateOptionAlertRequestEvent) ParseHTTPRequest(req *http.Request) error {
	if err := json.NewDecoder(req.Body).Decode(r); err != nil {
		return fmt.Errorf("CreateOptionAlertRequestEvent.ParseHTTPRequest: failed to decode request body: %w", err)
	}

	if r.AlertType == "" {
		return fmt.Errorf("CreateOptionAlertRequestEvent.ParseHTTPRequest: alert type is required")
	}

	if r.OptionSymbol == "" {
		return fmt.Errorf("CreateOptionAlertRequestEvent.ParseHTTPRequest: option symbol is required")
	}

	if r.Condition.Type == "" {
		return fmt.Errorf("CreateOptionAlertRequestEvent.ParseHTTPRequest: condition type is required")
	}

	if r.Condition.Direction == "" {
		return fmt.Errorf("CreateOptionAlertRequestEvent.ParseHTTPRequest: condition direction is required")
	}

	return nil
}

func (r *CreateOptionAlertRequestEvent) Validate(req *http.Request) error {
	if r.Condition.Value <= 0 {
		return fmt.Errorf("CreateOptionAlertRequestEvent.Validate: condition value must be greater than 0")
	}

	// todo: this is a hack to get around the fact that the request id is not being set
	// it should be set in middleware
	r.ID = uuid.New()

	return nil
}
