package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"slack-trading/src/models"
)

type ManualDatafeedUpdateRequest struct {
	RequestID uuid.UUID `json:"requestID"`
	Symbol    string    `json:"symbol"`
	Bid       float64   `json:"bid"`
	Ask       float64   `json:"ask"`
}

func (r *ManualDatafeedUpdateRequest) Validate(req *http.Request) error {
	if r.Symbol == "" {
		return fmt.Errorf("ManualDatafeedUpdateRequest.Validate: symbol was not set")
	}

	if r.Bid <= 0 {
		return fmt.Errorf("ManualDatafeedUpdateRequest.Validate: bid was not set")
	}

	if r.Ask <= 0 {
		return fmt.Errorf("ManualDatafeedUpdateRequest.Validate: ask was not set")
	}

	return nil
}

func (r *ManualDatafeedUpdateRequest) ParseHTTPRequest(req *http.Request) error {
	return json.NewDecoder(req.Body).Decode(r)
}

func (r *ManualDatafeedUpdateRequest) SetRequestID(id uuid.UUID) {
	r.RequestID = id
}

type ManualDatafeedUpdateResult struct {
	RequestID uuid.UUID   `json:"requestID"`
	UpdatedAt time.Time   `json:"updatedAt"`
	Tick      models.Tick `json:"tick"`
}

func (r *ManualDatafeedUpdateResult) GetRequestID() uuid.UUID {
	return r.RequestID
}

func NewManualDatafeedUpdateResult(requestID uuid.UUID, updatedAt time.Time, tick models.Tick) *ManualDatafeedUpdateResult {
	return &ManualDatafeedUpdateResult{RequestID: requestID, UpdatedAt: updatedAt, Tick: tick}
}
