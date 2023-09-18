package eventmodels

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net/http"
)

type NewSignalRequest struct {
	RequestID uuid.UUID `json:"requestID"`
	Name      string    `json:"name"`
}

func (r *NewSignalRequest) SetRequestID(id uuid.UUID) {
	r.RequestID = id
}

func (r *NewSignalRequest) ParseHTTPRequest(req *http.Request) error {
	if err := json.NewDecoder(req.Body).Decode(r); err != nil {
		return fmt.Errorf("NewSignalRequest.ParseHTTPRequest: failed to decode json: %w", err)
	}

	if r.Name == "" {
		return fmt.Errorf("GetStatsRequest.Validate: account name was not found")
	}

	return nil
}
