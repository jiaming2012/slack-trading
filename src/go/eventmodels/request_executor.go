package eventmodels

import "net/http"

type RequestExecutor interface {
	Serve(r *http.Request, req ApiRequest3, resultCh chan interface{}, errCh chan error)
}
