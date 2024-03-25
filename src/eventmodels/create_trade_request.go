package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// todo: make an APIRequestEvent struct

type CreateTradeRequest struct {
	Meta         *MetaData
	RequestID    uuid.UUID
	AccountName  string             `json:"AccountName"`
	StrategyName string             `json:"strategyName"`
	Timeframe    *int               `json:"timeframe"`
	Error        chan RequestError2 `json:"-"`
}

func (r *CreateTradeRequest) Wait() chan RequestError2 {
	return r.Error
}

func (r *CreateTradeRequest) GetMetaData() *MetaData {
	return r.Meta
}

func (r *CreateTradeRequest) ParseHTTPRequest(req *http.Request) error {
	if err := json.NewDecoder(req.Body).Decode(&r); err != nil {
		return fmt.Errorf("OpenTradeRequest.ParseHTTPRequest: failed to decode json: %w", err)
	}

	return nil
}

func (r *CreateTradeRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}

func (r *CreateTradeRequest) SetRequestID(id uuid.UUID) {
	r.RequestID = id
}

func NewOpenTradeRequest(requestID uuid.UUID, accountName string, strategyName string, timeframe *int) (*CreateTradeRequest, error) {
	req := &CreateTradeRequest{
		Meta:      &MetaData{ParentMeta: nil, RequestError: make(chan RequestError2)},
		RequestID: requestID, AccountName: accountName,
		StrategyName: strategyName,
		Timeframe:    timeframe,
		// Error:        make(chan EventError),
	}

	if err := req.Validate(nil); err != nil {
		return nil, err
	}

	return req, nil
}

func (r *CreateTradeRequest) Validate(request *http.Request) error {
	if len(r.AccountName) == 0 {
		return fmt.Errorf("validate: AccountName not set")
	}

	if len(r.StrategyName) == 0 {
		return fmt.Errorf("validate: strategyName not set")
	}

	if r.Timeframe != nil && *r.Timeframe <= 0 {
		return fmt.Errorf("validate: timeframe must be greater than zero")
	}

	return nil
}
