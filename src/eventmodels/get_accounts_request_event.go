package eventmodels

import "net/http"

type GetAccountsRequestEvent struct {
	BaseRequestEvent2
}

func (e *GetAccountsRequestEvent) Validate(r *http.Request) error {
	return nil
}

func (e *GetAccountsRequestEvent) ParseHTTPRequest(r *http.Request) error {
	return nil
}
