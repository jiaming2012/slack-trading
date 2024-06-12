package eventmodels

import "net/http"

type ApiRequest3 interface {
	ParseHTTPRequest(r *http.Request) error
	Validate(r *http.Request) error
}
