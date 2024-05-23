package eventproducers

import (
	"net/http"

	"slack-trading/src/eventmodels"
)

type ApiRequest3 interface {
	ParseHTTPRequest(r *http.Request) error
	Validate(r *http.Request) error
	ServeRequest(r *http.Request, serve eventmodels.ServeRequester) (chan interface{}, chan error)
}
