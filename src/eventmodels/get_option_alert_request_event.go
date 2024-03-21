package eventmodels

import (
	"net/http"
)

type GetOptionAlertRequestEvent struct {
	BaseRequstEvent
}

func (r *GetOptionAlertRequestEvent) ParseHTTPRequest(req *http.Request) error {
	return nil
}

func (r *GetOptionAlertRequestEvent) Validate(req *http.Request) error {
	return nil
}
