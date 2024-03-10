package eventmodels

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type GetStatsRequest struct {
	Meta        *MetaData `json:"meta"`
	RequestID   uuid.UUID `json:"requestID"`
	AccountName string    `json:"AccountName"`
}

func (r *GetStatsRequest) GetMetaData() *MetaData {
	return r.Meta
}

func (r *GetStatsRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}

func (r *GetStatsRequest) SetRequestID(id uuid.UUID) {
	r.RequestID = id
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
