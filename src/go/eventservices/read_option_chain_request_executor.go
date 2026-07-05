package eventservices

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/utils"
)

func Serve_ReadOptionChainRequestExecutor(s *models.ReadOptionChainRequestExecutor, r *http.Request, request models.ApiRequest3, projectDir string, resultCh chan map[string]interface{}, errorCh chan error) {
	req := request.(*models.ReadOptionChainRequest)

	bFindSpreads := false
	if r.URL.Path == "/options/spreads" {
		bFindSpreads = true
	}

	if req.EV != nil {
		// data, err := s.CollectData(r.Context(), req)
		now := time.Now()
		expirationGTE := now
		nextOptionsExpirationDate := utils.DeriveNextFriday(expirationGTE)
		// nextOptionsExpirationDate := utils.DeriveNextExpiration(expirationGTE, req.ExpirationsInDays)

		maxTickAge := time.Duration(6.5 * float64(time.Minute))
		data, err := s.OptionsDataFetcher.FetchOptionChainV1(req.Symbol, now, expirationGTE, nextOptionsExpirationDate, 0, 0, []int{}, maxTickAge, nil, nil)

		if err != nil {
			errorCh <- fmt.Errorf("tradier executer: %v: failed to collect data: %v", req.Symbol, err)
			return
		}
		go s.ServeWithParams(r.Context(), req, *data, bFindSpreads, projectDir, now, resultCh, errorCh)
	} else {
		// go s.serve(req, resultCh, errorCh)
		panic("not implemented")
	}
}
