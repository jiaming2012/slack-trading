package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ManualDatafeedUpdateRequest struct {
	BaseRequestEvent
	Symbol string  `json:"symbol"`
	Bid    float64 `json:"bid"`
	Ask    float64 `json:"ask"`
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
