package eventmodels

import "net/http"

type RequestExecutor interface {
	Serve(r *http.Request, req ApiRequest3, resultCh chan map[string]interface{}, errCh chan error)
}
