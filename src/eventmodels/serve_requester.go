package eventmodels

import "net/http"

type RequestExecutor interface {
	Serve(r *http.Request, req ApiRequest3) (chan map[string]interface{}, chan error)
}
