package eventmodels

import "net/http"

type ServeRequester interface {
	ServeRequest(r *http.Request) (chan interface{}, chan error)
}