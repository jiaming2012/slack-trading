package eventmodels

import (
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"slack-trading/src/models"
	"time"
)

type ManualDatafeedUpdateRequest struct {
	RequestID uuid.UUID `json:"requestID"`
	Symbol    string    `json:"symbol"`
	Bid       float64   `json:"bid"`
	Ask       float64   `json:"ask"`
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
