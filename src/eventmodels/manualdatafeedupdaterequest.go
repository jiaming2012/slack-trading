package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

type ManualDatafeedUpdateRequest struct {
	Meta      *MetaData `json:"meta"`
	RequestID uuid.UUID `json:"requestID"`
	Symbol    string    `json:"symbol"`
	Bid       float64   `json:"bid"`
	Ask       float64   `json:"ask"`
}

func (r *ManualDatafeedUpdateRequest) GetMetaData() *MetaData {
	return r.Meta
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

func (r *ManualDatafeedUpdateRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}
