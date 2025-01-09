package eventservices

import (
	"net/http"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type AppVersion struct{}

func (m *AppVersion) Serve(r *http.Request, apiRequest eventmodels.ApiRequest3, resultCh chan interface{}, errCh chan error) {
	resultCh <- &eventmodels.AppVersionResponseDTO{
		Version: "2.4.14",
	}
}
