package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

type CreateAccountStrategyRequestEvent struct {
	AccountsRequestHeader
	Strategy StrategiesPostRequest `json:"strategy"`
}

func (r *CreateAccountStrategyRequestEvent) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName: AccountsStreamName,
		EventName:  CreateAccountStrategyRequestEventName,
	}
}

func (r *CreateAccountStrategyRequestEvent) Validate(request *http.Request) error {
	if r.AccountName == "" {
		return fmt.Errorf("CreateAccountsStrategiesRequestEvent.Validate: account name was not set")
	}

	return nil
}

func (r *CreateAccountStrategyRequestEvent) ParseHTTPRequest(request *http.Request) error {
	// Get the account name from the path parameters
	vars := mux.Vars(request)
	accountName, found := vars["accountName"]
	if !found {
		return fmt.Errorf("CreateAccountsStrategiesRequestEvent.Validate: could not find account name in path parameters")
	}

	// Set request variables from path parameters
	r.AccountName = accountName

	// Decode the JSON request body into the map
	var data map[string]json.RawMessage
	if err := json.NewDecoder(request.Body).Decode(&data); err != nil {
		return fmt.Errorf("CreateAccountsStrategiesRequestEvent.ParseHTTPRequest: failed to decode json: %w", err)
	}

	var strategy json.RawMessage
	if strategy, found = data["strategy"]; !found {
		return fmt.Errorf("CreateAccountsStrategiesRequestEvent.ParseHTTPRequest: strategy was not found")
	}

	// Set request variables from json
	if err := json.Unmarshal(strategy, &r.Strategy); err != nil {
		return fmt.Errorf("CreateAccountsStrategiesRequestEvent.ParseHTTPRequest: failed to decode strategy: %w", err)
	}

	return nil
}
