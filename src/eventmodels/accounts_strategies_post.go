package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"slack-trading/src/models"
)

type AccountsStrategiesPostRequest struct {
	AccountsRequestHeader
	Strategy StrategiesPostRequest `json:"strategy"`
}

func (r *AccountsStrategiesPostRequest) Validate(request *http.Request) error {
	if r.AccountName == "" {
		return fmt.Errorf("AccountsStrategiesPostRequest.Validate: account name was not set")
	}

	return nil
}

func (r *AccountsStrategiesPostRequest) ParseHTTPRequest(request *http.Request) error {
	// Get the account name from the path parameters
	vars := mux.Vars(request)
	accountName, found := vars["accountName"]
	if !found {
		return fmt.Errorf("AccountsStrategiesPostRequest.Validate: could not find account name in path parameters")
	}

	// Set request variables from path parameters
	r.AccountName = accountName

	// Decode the JSON request body into the map
	var data map[string]json.RawMessage
	if err := json.NewDecoder(request.Body).Decode(&data); err != nil {
		return fmt.Errorf("AccountsStrategiesPostRequest.ParseHTTPRequest: failed to decode json: %w", err)
	}

	var strategy json.RawMessage
	if strategy, found = data["strategy"]; !found {
		return fmt.Errorf("AccountsStrategiesPostRequest.ParseHTTPRequest: strategy was not found")
	}

	// Set request variables from json
	if err := json.Unmarshal(strategy, &r.Strategy); err != nil {
		return fmt.Errorf("AccountsStrategiesPostRequest.ParseHTTPRequest: failed to decode strategy: %w", err)
	}

	return nil
}

func (r *AccountsStrategiesPostRequest) SetRequestID(id uuid.UUID) {
	r.RequestID = id
}

func (r *AccountsStrategiesPostRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}

type AccountsStrategiesPostResponse struct {
	AccountsRequestHeader
	Strategy *models.Strategy `json:"strategy"`
}
