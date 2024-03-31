package eventmodels

import (
	"net/http"
)

type GetStrategiesRequestEvent struct {
	BaseRequestEvent
}

func (e *GetStrategiesRequestEvent) Validate(r *http.Request) error {
	return nil
}

func (e *GetStrategiesRequestEvent) ParseHTTPRequest(r *http.Request) error {
	return nil
}
