package eventmodels

import (
	"net/http"
)

type GetOptionAlertRequestEvent struct {
	BaseRequestEvent
}

func (r *GetOptionAlertRequestEvent) ParseHTTPRequest(req *http.Request) error {
	return nil
}

func (r *GetOptionAlertRequestEvent) Validate(req *http.Request) error {
	return nil
}
