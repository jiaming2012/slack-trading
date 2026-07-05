package eventservices

import (
	"net/http"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

type AppVersion struct{}

func GetAppVersion() string {
	return "3.26.0"
}

func (m *AppVersion) Serve(r *http.Request, apiRequest models.ApiRequest3, resultCh chan interface{}, errCh chan error) {
	resultCh <- &models.AppVersionResponseDTO{
		Version: GetAppVersion(),
	}
}
