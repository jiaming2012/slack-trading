package eventproducers

import (
	"net/http"
	"slack-trading/src/eventmodels"
)

type ApiRequest2 interface {
	ParseHTTPRequest(r *http.Request) error
	Validate(r *http.Request) error
	GetMetaData() *eventmodels.MetaData
	SetMetaData(*eventmodels.MetaData)
}
