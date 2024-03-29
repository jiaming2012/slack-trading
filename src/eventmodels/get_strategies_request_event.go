package eventmodels

import (
	"net/http"
)

type GetStrategiesRequestEvent struct {
	BaseRequestEvent2
}

func (e *GetStrategiesRequestEvent) Validate(r *http.Request) error {
	return nil
}

func (e *GetStrategiesRequestEvent) ParseHTTPRequest(r *http.Request) error {
	return nil
}
