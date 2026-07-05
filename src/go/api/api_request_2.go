package api

import (
	"net/http"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

type ApiRequest2 interface {
	ParseHTTPRequest(r *http.Request) error
	Validate(r *http.Request) error
	GetMetaData() *models.MetaData
	SetMetaData(*models.MetaData)
}
