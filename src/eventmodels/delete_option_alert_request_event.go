package eventmodels

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

type DeleteOptionAlertRequestEvent struct {
	BaseRequestEvent
	DeleteOptionAlertRequestDTO
	AlertID uuid.UUID
}

func (r *DeleteOptionAlertRequestEvent) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName: OptionAlertsStreamName,
		EventName:  DeleteOptionAlertRequestEventName,
	}
}

func (r *DeleteOptionAlertRequestEvent) ParseHTTPRequest(req *http.Request) error {
	queryParams := req.URL.Query()
	r.ID = queryParams.Get("id")
	if r.ID == "" {
		return fmt.Errorf("DeleteOptionAlertRequestEvent.ParseHTTPRequest: id is required")
	}

	return nil
}

func (r *DeleteOptionAlertRequestEvent) Validate(req *http.Request) error {
	id, err := uuid.Parse(r.ID)
	if err != nil {
		return fmt.Errorf("NewDeleteOptionAlertRequestEvent: invalid id: %w", err)
	}

	r.AlertID = id
	return nil
}
