package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"slack-trading/src/models"
)

type GetAccountsRequestEvent struct {
	RequestHeader
}

func (e *GetAccountsRequestEvent) GetMetaData() *MetaData {
	return e.Meta
}

func (e *GetAccountsRequestEvent) Validate(r *http.Request) error {
	return nil
}

type CreateAccountRequestEvent struct {
	Meta         *MetaData           `json:"meta"`
	RequestID    uuid.UUID           `json:"requestID"`
	Name         string              `json:"name"`
	Balance      float64             `json:"balance"`
	DatafeedName models.DatafeedName `json:"datafeedName"`
}

func (e *CreateAccountRequestEvent) GetMetaData() *MetaData {
	return e.Meta
}

func (e *CreateAccountRequestEvent) GetRequestID() uuid.UUID {
	return e.RequestID
}

func (e *CreateAccountRequestEvent) SetRequestID(id uuid.UUID) {
	e.RequestID = id
}

func (e *CreateAccountRequestEvent) Validate(r *http.Request) error {
	if e.Name == "" {
		return fmt.Errorf("CreateAccountRequestEvent.Validate: name was not set")
	}

	if e.Balance <= 0 {
		return fmt.Errorf("CreateAccountRequestEvent.Validate: balance was not set")
	}

	if e.DatafeedName == "" {
		return fmt.Errorf("CreateAccountRequestEvent.Validate: datafeedName was not set")
	}

	return nil
}

func (e *CreateAccountRequestEvent) ParseHTTPRequest(r *http.Request) error {
	var values map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&values); err != nil {
		return fmt.Errorf("PostAccountsRequestEvent.ParseHTTPRequest: failed to decode json: %w", err)
	}

	if payload, found := values["name"]; found {
		if val, ok := payload.(string); ok {
			e.Name = val
		}
	} else {
		return fmt.Errorf("PostAccountsRequestEvent.ParseHTTPRequest: name was not found")
	}

	if payload, found := values["balance"]; found {
		if val, ok := payload.(float64); ok {
			e.Balance = val
		}
	} else {
		return fmt.Errorf("PostAccountsRequestEvent.ParseHTTPRequest: balance was not found")
	}

	if payload, found := values["datafeedName"]; found {
		if val, ok := payload.(string); ok {
			e.DatafeedName = models.DatafeedName(val)
		}
	} else {
		return fmt.Errorf("PostAccountsRequestEvent.ParseHTTPRequest: datafeedName was not found")
	}

	return nil
}

func (e *GetAccountsRequestEvent) ParseHTTPRequest(r *http.Request) error {
	return nil
}

func (e *GetAccountsRequestEvent) SetRequestID(id uuid.UUID) {
	e.RequestID = id
}

type GetAccountsResponseEvent struct {
	Meta      *MetaData         `json:"meta"`
	RequestID uuid.UUID         `json:"requestID"`
	Accounts  []*models.Account `json:"accounts"`
}

func (e *GetAccountsResponseEvent) GetMetaData() *MetaData {
	return e.Meta
}

func (e *GetAccountsResponseEvent) GetRequestID() uuid.UUID {
	return e.RequestID
}

type CreateAccountResponseEvent struct {
	Meta      *MetaData       `json:"meta"`
	RequestID uuid.UUID       `json:"requestID"`
	Account   *models.Account `json:"account"`
}

func (e *CreateAccountResponseEvent) GetMetaData() *MetaData {
	return e.Meta
}

func (e *CreateAccountResponseEvent) GetRequestID() uuid.UUID {
	return e.RequestID
}

// todo: remove this??
// type AddAccountRequestEvent struct {
// 	Name              string
// 	Balance           float64
// 	MaxLossPercentage float64
// 	PriceLevelsInput  [][3]float64
// 	DatafeedName      models.DatafeedName
// }

// func (e *AddAccountRequestEvent) GetRequestID() uuid.UUID {
// 	return uuid.New()
// }

type AddAccountResponseEvent struct {
	RequestID uuid.UUID
	Account   *models.Account
}

func (e *AddAccountResponseEvent) GetRequestID() uuid.UUID {
	return e.RequestID
}
