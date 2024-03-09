package eventmodels

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"slack-trading/src/models"
)

type FetchTradesRequest struct {
	RequestID    uuid.UUID `json:"requestID"`
	AccountName  string    `json:"accountName"`
	StrategyName *string   `json:"strategyName"`
}

func (r *FetchTradesRequest) ParseHTTPRequest(req *http.Request) error {
	vars := mux.Vars(req)
	accountName, found := vars["accountName"]
	if !found {
		return fmt.Errorf("could not find accountName in request params")
	}

	r.AccountName = accountName
	return nil
}

func (r *FetchTradesRequest) Validate(request *http.Request) error {
	if len(r.AccountName) == 0 {
		return fmt.Errorf("validate: AccountName not set")
	}

	return nil
}

func (r *FetchTradesRequest) SetRequestID(id uuid.UUID) {
	r.RequestID = id
}

func (r *FetchTradesRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}

func NewFetchTradesRequest(requestID uuid.UUID, accountName string, strategyName *string) *FetchTradesRequest {
	return &FetchTradesRequest{RequestID: requestID, AccountName: accountName, StrategyName: strategyName}
}

type FetchTradesResult struct {
	RequestID uuid.UUID             `json:"requestID"`
	Trades    []*models.TradeLevels `json:"trades"`
}

func (r *FetchTradesResult) GetRequestID() uuid.UUID {
	return r.RequestID
}

func NewFetchTradesResult(requestID uuid.UUID, trades []*models.TradeLevels) *FetchTradesResult {
	return &FetchTradesResult{RequestID: requestID, Trades: trades}
}
