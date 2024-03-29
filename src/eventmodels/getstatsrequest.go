package eventmodels

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

type GetStatsRequest struct {
	BaseRequestEvent2
	AccountName string `json:"AccountName"`
}

func (r *GetStatsRequest) ParseHTTPRequest(request *http.Request) error {
	return nil
}

func (r *GetStatsRequest) Validate(request *http.Request) error {
	vars := mux.Vars(request)
	accountName, found := vars["accountName"]
	if !found {
		return fmt.Errorf("GetStatsRequest.Validate: could not find account name in path parameters")
	}

	r.AccountName = accountName

	if r.AccountName == "" {
		return fmt.Errorf("GetStatsRequest.Validate: account name was not set")
	}

	return nil
}
