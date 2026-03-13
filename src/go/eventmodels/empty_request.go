package eventmodels

import "net/http"

type EmptyRequest struct{}

func (req *EmptyRequest) ParseHTTPRequest(r *http.Request) error {
	return nil
}

func (req *EmptyRequest) Validate(r *http.Request) error {
	return nil
}
