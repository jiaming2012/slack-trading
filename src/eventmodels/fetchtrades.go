package eventmodels

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type FetchTradesRequest struct {
	BaseRequestEvent
	AccountName  string  `json:"accountName"`
	StrategyName *string `json:"strategyName"`
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

func NewFetchTradesRequest(requestID uuid.UUID, accountName string, strategyName *string) *FetchTradesRequest {
	return &FetchTradesRequest{
		BaseRequestEvent: BaseRequestEvent{Meta: MetaData{RequestID: requestID}},
		AccountName:      accountName,
		StrategyName:     strategyName,
	}
}

type FetchTradesResult struct {
	BaseResponseEvent2
	Trades []*TradeLevels `json:"trades"`
}

func NewFetchTradesResult(requestID uuid.UUID, trades []*TradeLevels) *FetchTradesResult {
	return &FetchTradesResult{BaseResponseEvent2: BaseResponseEvent2{Meta: MetaData{RequestID: requestID}}, Trades: trades}
}
