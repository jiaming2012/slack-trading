package eventmodels

import "net/http"

type GetAccountsRequestEvent struct {
	RequestHeader
}

func (e *GetAccountsRequestEvent) Validate(r *http.Request) error {
	return nil
}
