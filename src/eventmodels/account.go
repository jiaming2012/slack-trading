package eventmodels

import (
	"github.com/google/uuid"
	"net/http"
	"slack-trading/src/models"
)

type GetAccountsRequestEvent struct {
	RequestID uuid.UUID `json:"requestID"`
}

func (e *GetAccountsRequestEvent) ParseHTTPRequest(r *http.Request) error {
	return nil
}

func (e *GetAccountsRequestEvent) SetRequestID(id uuid.UUID) {
	e.RequestID = id
}

type GetAccountsResponseEvent struct {
	RequestID uuid.UUID        `json:"requestID"`
	Accounts  []models.Account `json:"accounts"`
}

func (e *GetAccountsResponseEvent) GetRequestID() uuid.UUID {
	return e.RequestID
}

type AddAccountRequestEvent struct {
	Name              string
	Balance           float64
	MaxLossPercentage float64
	PriceLevelsInput  [][3]float64
}

type AddAccountResponseEvent struct {
	RequestID uuid.UUID
	Account   models.Account
}

func (e *AddAccountResponseEvent) GetRequestID() uuid.UUID {
	return e.RequestID
}
