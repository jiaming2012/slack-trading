package eventmodels

import (
	"net/http"

	"github.com/google/uuid"

	"slack-trading/src/models"
)

type GetStrategiesRequestEvent struct {
	RequestHeader
}

func (e *GetStrategiesRequestEvent) Validate(r *http.Request) error {
	return nil
}

func (e *GetStrategiesRequestEvent) ParseHTTPRequest(r *http.Request) error {
	return nil
}

func (e *GetStrategiesRequestEvent) SetRequestID(id uuid.UUID) {
	e.RequestID = id
}

type GetStrategiesResponseEvent struct {
	RequestHeader
	Strategies []*models.Strategy `json:"strategies"`
}

func (e *GetStrategiesResponseEvent) GetRequestID() uuid.UUID {
	return e.RequestID
}
